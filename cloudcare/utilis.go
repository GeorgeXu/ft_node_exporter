package cloudcare

import (
	"fmt"
	"io/ioutil"
	"itos/logio"
	"log"
	"os"
	"path"

	"github.com/prometheus/node_exporter/cfg"
)

func DumpPID() error {
	pid := os.Getpid()

	pidfile := fmt.Sprintf("%s/%s.pid", cfg.InstallDir+cfg.ProbeName, cfg.ProbeName)
	return ioutil.WriteFile(pidfile, []byte(fmt.Sprintf("%d", pid)), 0664)
}

func SetLog(f string) (*logio.RotateWriter, error) {

	err := os.MkdirAll(path.Dir(f), os.ModePerm)
	if err != nil {
		log.Fatalf("[fatal] %s", err.Error())
	}

	rw, err := logio.New(f)
	if err != nil {
		log.Printf("[error] %s", err.Error())
		return nil, err
	}

	log.SetOutput(rw)

	return rw, nil
}
