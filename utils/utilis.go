package utils

import (
	"fmt"
	"io/ioutil"
	"itos/logio"
	"log"
	"os"
	"path"
)

func DumpPID(installDir string, probeName string) error {
	pid := os.Getpid()

	pidfile := fmt.Sprintf("%s%s.pid", installDir, probeName)
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
