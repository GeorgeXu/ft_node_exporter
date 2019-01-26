package cloudcare

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/node_exporter/cfg"
	"github.com/prometheus/prometheus/prompb"
)

// String constants for instrumentation.
const (

	// We track samples in/out and how long pushes take using an Exponentially
	// Weighted Moving Average.
	ewmaWeight          = 0.2
	shardUpdateDuration = 10 * time.Second

	// Allow 30% too many shards before scaling down.
	shardToleranceFraction = 0.3
)

type StorageClient interface {
	// Store stores the given samples in the remote storage.
	Store(context.Context, *prompb.WriteRequest) error
	// Name identifies the remote storage implementation.
	Name() string
}

type QueueManager struct {
	flushDeadline time.Duration
	client        StorageClient
	queueName     string

	shardsMtx   sync.Mutex
	shards      *shards
	numShards   int
	reshardChan chan int
	quit        chan struct{}
	wg          sync.WaitGroup
	dropped     int64

	integralAccumulator float64
}

func dumpSamples(samples model.Samples) (string, error) {
	b, err := json.Marshal(samples)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func newQueueManager(client StorageClient, flushDeadline time.Duration) *QueueManager {

	qm := &QueueManager{
		flushDeadline: flushDeadline,
		client:        client,
		queueName:     client.Name(),

		numShards:   1,
		reshardChan: make(chan int),
		quit:        make(chan struct{}),
	}

	qm.shards = qm.newShards(qm.numShards)

	return qm
}

// Append queues a sample to be sent to the remote storage. It drops the
// sample on the floor if the queue is full.
// Always returns nil.
func (qm *QueueManager) Append(s *model.Sample) error {
	snew := *s
	snew.Metric = s.Metric.Clone()

	if snew.Metric == nil {
		return nil
	}

	qm.shardsMtx.Lock()
	ok := qm.shards.enqueue(&snew)
	qm.shardsMtx.Unlock()
	if !ok {
		qm.dropped++
		log.Printf("[warn] sample queue full, total dropped %d samples", qm.dropped)
	}

	return nil
}

// Start the queue manager sending samples to the remote storage.
// Does not block.
func (qm *QueueManager) Start() {

	qm.shardsMtx.Lock()
	defer qm.shardsMtx.Unlock()
	qm.shards.start()
}

// Stop stops sending samples to the remote storage and waits for pending
// sends to complete.
func (qm *QueueManager) Stop() {
	log.Printf("[info] Stopping remote storage...")
	close(qm.quit)
	qm.wg.Wait()

	qm.shardsMtx.Lock()
	defer qm.shardsMtx.Unlock()
	qm.shards.stop(qm.flushDeadline)

	log.Printf("[info] Remote storage stopped.")
}

type shards struct {
	qm      *QueueManager
	queues  []chan *model.Sample
	done    chan struct{}
	running int32
	ctx     context.Context
	cancel  context.CancelFunc
}

func (qm *QueueManager) newShards(numShards int) *shards {
	queues := make([]chan *model.Sample, numShards)
	for i := 0; i < numShards; i++ {
		queues[i] = make(chan *model.Sample, cfg.Cfg.QueueCfg[`capacity`])
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &shards{
		qm:      qm,
		queues:  queues,
		done:    make(chan struct{}),
		running: int32(numShards),
		ctx:     ctx,
		cancel:  cancel,
	}
	return s
}

func (s *shards) start() {
	for i := 0; i < len(s.queues); i++ {
		go s.runShard(i)
	}
}

func (s *shards) stop(deadline time.Duration) {
	// Attempt a clean shutdown.
	for _, shard := range s.queues {
		close(shard)
	}

	select {
	case <-s.done:
		return
	case <-time.After(deadline):
		log.Printf("[error] Failed to flush all samples on shutdown")
	}

	// Force an unclean shutdown.
	s.cancel()
	<-s.done
	return
}

func (s *shards) enqueue(sample *model.Sample) bool {

	fp := sample.Metric.FastFingerprint()
	shard := uint64(fp) % uint64(len(s.queues))

	select {
	case s.queues[shard] <- sample:
		return true
	default:
		return false
	}
}

func (s *shards) runShard(i int) {
	defer func() {
		if atomic.AddInt32(&s.running, -1) == 0 {
			close(s.done)
		}
	}()

	queue := s.queues[i]

	// Send batches of at most MaxSamplesPerSend samples to the remote storage.
	// If we have fewer samples than that, flush them out after a deadline
	// anyways.
	pendingSamples := model.Samples{}

	log.Printf("[info] run shard %d...", i)
	batchSize := cfg.Cfg.QueueCfg[`max_samples_per_send`]
	deadline := time.Duration(cfg.Cfg.QueueCfg[`batch_send_deadline`]) * time.Second
	timer := time.NewTimer(deadline)

	stop := func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}
	defer stop()

	total := int64(0)

	for {
		select {
		case <-s.ctx.Done():
			return

		case sample, ok := <-queue:
			total++

			if !ok {
				if len(pendingSamples) > 0 {
					log.Printf("[debug] Flushing %d samples to remote storage, total %d", len(pendingSamples), total)
					s.sendSamples(pendingSamples)
					log.Printf("[debug] Done flushing.")
				}
				return
			}

			AddTags(sample)

			pendingSamples = append(pendingSamples, sample)

			if len(pendingSamples) >= batchSize {

				log.Printf("[debug] total samples: %d", total)

				s.sendSamples(pendingSamples[:batchSize])
				pendingSamples = pendingSamples[batchSize:]

				stop()
				timer.Reset(deadline)
			}

		case <-timer.C:
			if len(pendingSamples) > 0 {
				log.Printf("[debug] send %d samples on timer, total: %d", len(pendingSamples), total)
				s.sendSamples(pendingSamples)
				pendingSamples = pendingSamples[:0]
			}

			timer.Reset(deadline)
		}
	}
}

func (s *shards) sendSamples(samples model.Samples) {
	start := time.Now()
	s.sendSamplesWithBackoff(samples)
	log.Printf("[debug] send samples elapsed %v", time.Since(start))
}

// send samples to the remote storage with backoff for recoverable errors.
func (s *shards) sendSamplesWithBackoff(samples model.Samples) {
	req := ToWriteRequest(samples)

	for retries := cfg.Cfg.QueueCfg[`max_retries`]; retries > 0; retries-- {

		err := s.qm.client.Store(s.ctx, req)

		if err == nil {
			return
		}

		log.Printf("[error] Error sending samples to remote storage samples: %s", err)
	}
}
