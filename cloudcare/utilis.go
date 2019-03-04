package cloudcare

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/prometheus/node_exporter/cfg"
)

func DumpPID() error {
	pid := os.Getpid()

	pidfile := fmt.Sprintf("%s/%s.pid", cfg.InstallDir+cfg.ProbeName, cfg.ProbeName)
	return ioutil.WriteFile(pidfile, []byte(fmt.Sprintf("%d", pid)), 0664)
}
