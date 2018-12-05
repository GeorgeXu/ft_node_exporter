// +build ignore

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/prometheus/node_exporter/git"

	"itos/agent/storage"
	"itos/tunnel"
)

var (
	flagParallel = flag.Int("parallel", runtime.NumCPU(), "number of commands to run in parallel")
	flagBinary   = flag.String("binary", "", "binary name to build")
	flagName     = flag.String("name", *flagBinary, "same as -binary")
	flagBuildDir = flag.String("build-dir", "build", "output of build files")
	flagMain     = flag.String(`main`, `main.go`, `binary build entry`)
	flagCGO      = flag.Bool(`cgo`, false, `enable CGO or not`)

	flagKodoHost     = flag.String("kodo-host", "", "")
	flagDownloadAddr = flag.String("download-addr", "", "")
	flagSsl          = flag.Int("ssl", 0, "")
	flagPort         = flag.Int("port", 0, "")
	flagPubDir       = flag.String("pub-dir", "pub", "")

	flagArchs    = flag.String("archs", "linux/amd64", "os archs")
	flagArchAll  = flag.Bool("all-arch", false, "build for all OS")
	flagShowArch = flag.Bool(`show-arch`, false, `show all OS`)

	flagRelease = flag.String(`release`, ``, `build for local/test/alpha/preprod/release`)

	flagReleaseAgent = flag.String(`release-agent`, ``, `upload build agent to OSS: local/test/alpha/release/preprod`)

	workDir string
	homeDir string

	curVersion []byte

	osarches = []string{
		"linux/386",
		"linux/amd64",

		"windows/386",
		"windows/amd64",
		"darwin/386",
		"darwin/amd64",

		"linux/arm",
		"linux/arm64",
		"freebsd/386",
		"freebsd/amd64",
		"freebsd/arm",
		"netbsd/386",
		"netbsd/amd64",
		"netbsd/arm",
		"openbsd/386",
		"openbsd/amd64",
		"plan9/386",
		"plan9/amd64",
		"solaris/amd64",
		"linux/mips",
		"linux/mipsle",
	}
)

type versionDesc struct {
	Version   string `json:"version"`
	Date      string `json:"date"`
	ChangeLog string `json:"changeLog"` // TODO: add release note
}

func init() {

	var err error
	workDir, err = os.Getwd()
	if err != nil {
		log.Fatalf("%v", err)
	}

	workDir, err = filepath.Abs(workDir)
	if err != nil {
		log.Fatalf("%v", err)
	}
}

func runEnv(args, env []string) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}

	log.Printf("%s %s", strings.Join(env, " "), strings.Join(args, " "))
	err := cmd.Run()
	if err != nil {
		log.Fatalf("failed to run %v: %v", args, err)
	}
}

func run(args ...string) {
	runEnv(args, nil)
}

func compileArch(bin, goos, goarch, dir string) {
	log.Printf("building %s.%s/%s(%s)...", bin, goos, goarch, *flagMain)

	output := path.Join(dir, bin)
	if goos == "windows" {
		output += ".exe"
	}

	args := []string{
		"go", "build",
		"-o", output,
		*flagMain,
	}

	env := []string{
		"GOOS=" + goos,
		"GOARCH=" + goarch,
	}

	if *flagCGO {
		env = append(env, "CGO_ENABLED=1")
	} else {
		env = append(env, "CGO_ENABLED=0")
	}

	runEnv(args, env)
}

func compile() {
	start := time.Now()
	var wg sync.WaitGroup

	done := make(chan int, *flagParallel)
	defer close(done)

	compileTask := func(bin, goos, goarch, dir string) {
		defer wg.Done()
		compileArch(bin, goos, goarch, dir)
		done <- 0
	}

	jobs := 0

	var archs []string

	if *flagArchAll {
		archs = osarches
	} else {
		archs = strings.Split(*flagArchs, ",")
	}

	for _, arch := range archs {

	wait:
		select {
		case <-done:
			jobs--
		default:
		}

		if jobs >= *flagParallel {
			time.Sleep(time.Second)
			goto wait
		}

		parts := strings.Split(arch, "/")
		if len(parts) != 2 {
			log.Fatalf("invalid arch %q", parts)
		}

		goos, goarch := parts[0], parts[1]

		userGoos := goos
		if goos == "darwin" {
			userGoos = "osx"
		}

		dir := fmt.Sprintf("build/%s-%s-%s", *flagName, userGoos, goarch)

		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			log.Fatalf("failed to mkdir: %v", err)
		}

		dir, err = filepath.Abs(dir)
		if err != nil {
			log.Fatal("[fatal] %v", err)
		}

		wg.Add(1)
		jobs++
		go compileTask(*flagBinary, goos, goarch, dir)
	}

	wg.Wait()
	log.Printf("build %d binaries in %v", len(osarches), time.Since(start))
}

func getCurrentVersionInfo(url string) *versionDesc {

	log.Printf("get current online version: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("[fatal] %s", err.Error())
	}

	if resp.StatusCode != 200 {
		return nil
	}

	defer resp.Body.Close()
	info, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("current online version: %s", string(info))
	var vd versionDesc
	if err := json.Unmarshal(info, &vd); err != nil {
		log.Fatal(err)
	}
	return &vd
}

func releaseAgent() {
	var ak, sk, bucket, objPath, ossHost, prefix string

	// 在你本地设置好这些 oss-key 环境变量
	switch *flagReleaseAgent {
	case `test`, `local`, `release`, `preprod`, `alpha`:
		tag := strings.ToUpper(*flagReleaseAgent)
		ak = os.Getenv(tag + "_KODO_AGENT_OSS_ACCESS_KEY")
		sk = os.Getenv(tag + "_KODO_AGENT_OSS_SECRET_KEY")
		bucket = os.Getenv(tag + "_KODO_AGENT_OSS_BUCKET")
		objPath = os.Getenv(tag + "_KODO_AGENT_OSS_PATH")
		ossHost = os.Getenv(tag + "_KODO_AGENT_OSS_HOST")
	default:
		log.Fatalf("unknown release type: %s", *flagReleaseAgent)
	}

	prefix = path.Join(*flagPubDir, *flagReleaseAgent)

	if ak == "" || sk == "" {
		log.Fatal("[fatal] oss access key or secret key missing")
	}

	storage.DefaultOssOption = &tunnel.OssOption{
		Host:      ossHost,
		Bucket:    bucket,
		AccessKey: ak,
		SecretKey: sk,
		Path:      objPath,
	}

	oc, err := storage.NewOssCli()
	if err != nil {
		log.Fatalf("[fatal] %s", err)
	}

	// 请求线上的 agent 版本信息
	url := fmt.Sprintf("http://%s.%s/%s", bucket, ossHost, `version`)
	curVd := getCurrentVersionInfo(url)

	if curVd != nil {
		vOld := strings.Split(curVd.Version, `-`)
		vCur := strings.Split(git.Version, `-`)
		if vOld[0] == vCur[0] && vOld[1] == vCur[1] && vOld[2] == vCur[2] {
			log.Printf("[warn] Current OSS agent verison is the newest (%s <=> %s). Exit now.", curVd.Version, git.Version)
			os.Exit(0)
		}

		installObj := path.Join(objPath, "install.sh")
		installObjOld := path.Join(objPath, fmt.Sprintf("install-%s-%s.sh", vOld[0], vOld[1]))

		oc.Move(installObj, installObjOld)
	}

	gzName := fmt.Sprintf("%s-%s.tar.gz", *flagName, string(curVersion))
	objs := map[string]string{
		path.Join(prefix, gzName):       path.Join(objPath, gzName),
		path.Join(prefix, `install.sh`): path.Join(objPath, `install.sh`),
		path.Join(prefix, `version`):    path.Join(objPath, `version`),
	}

	for k, v := range objs {

		if err := oc.Upload(k, v); err != nil {
			log.Fatal(err)
		}
	}

	log.Println("Done :)")
}

func main() {

	var err error

	flag.Parse()

	log.SetFlags(log.Lshortfile | log.LstdFlags)

	// 获取当前版本信息, 形如: v3.0.0-42-g3ed424a
	curVersion, err = exec.Command("git", []string{`describe`, `--always`, `--tags`}...).Output()
	if err != nil {
		log.Fatal(err)
	}

	curVersion = bytes.TrimSpace(curVersion)

	if *flagReleaseAgent != "" {
		releaseAgent()
		return
	}

	gitsha1, err := exec.Command("git", []string{`rev-parse`, `--short`, `HEAD`}...).Output()
	if err != nil {
		log.Fatal(err)
	}

	dateStr, err := exec.Command("date", []string{"+'%Y-%m-%d %H:%M:%S'"}...).Output()
	if err != nil {
		log.Fatal(err)
	}

	golang, err := exec.Command("go", []string{"version"}...).Output()
	if err != nil {
		log.Fatal(err)
	}

	lastNCommits, err := exec.Command("git", []string{`log`, `-n`, `8`}...).Output()
	if err != nil {
		log.Fatal(err)
	}

	buildInfo := fmt.Sprintf(`// THIS FILE IS GENERATED BY make.go, DO NOT EDIT IT.
package git
const (
	Sha1 string = "%s"
	BuildAt string = "%s"
	Version string = "%s"
	Golang string = "%s"
)`,
		bytes.TrimSpace(gitsha1),

		// 输出会带有 ' 字符, 剪掉之
		bytes.Replace(bytes.TrimSpace(dateStr), []byte("'"), []byte(""), -1),

		// 移除此处的 `v' 前缀.  前端的版本号判断机制容不下这个前缀
		bytes.TrimSpace(curVersion[1:]),
		bytes.TrimSpace(golang),
	)

	// create git/git.go
	ioutil.WriteFile(`git/git.go`, []byte(buildInfo), 0666)

	if *flagKodoHost != "" { // build agent

		// create version info
		vd := &versionDesc{
			Version:   string(bytes.TrimSpace(curVersion[1:])),
			Date:      string(bytes.TrimSpace(dateStr)),
			ChangeLog: string(bytes.TrimSpace(lastNCommits)),
		}

		outdir := path.Join(*flagPubDir, "test")

		switch *flagRelease {
		case `test`:
			// default

		case `local`:
			outdir = path.Join(*flagPubDir, "local")

		case `preprod`:
			outdir = path.Join(*flagPubDir, "preprod")

		case `release`:
			outdir = path.Join(*flagPubDir, "release")

		case `alpha`:
			outdir = path.Join(*flagPubDir, "alpha")
		default:
			log.Fatalf("invalid release flag: %s", *flagRelease)
		}

		versionInfo, _ := json.Marshal(vd)
		ioutil.WriteFile(path.Join(outdir, `version`), versionInfo, 0666)

		// create install.sh script
		type Install struct {
			KodoHost     string
			Name         string
			DownloadAddr string
			Version      string
			Ssl          int
			Port         int
		}

		install := &Install{
			KodoHost:     *flagKodoHost,
			DownloadAddr: *flagDownloadAddr,
			Name:         *flagName,
			Version:      string(curVersion),
			Ssl:          *flagSsl,
			Port:         *flagPort,
		}

		log.Printf("[debug] %+#v", install)

		txt, err := ioutil.ReadFile("install.template")
		if err != nil {
			log.Fatal(err)
		}

		t := template.New("")
		t, err = t.Parse(string(txt))
		if err != nil {
			log.Fatal(err)
		}

		fd, err := os.OpenFile(path.Join(outdir, `install.sh`), os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}

		defer fd.Close()
		err = t.Execute(fd, install)
		if err != nil {
			log.Fatal(err)
		}
	} // endof build agent

	if *flagShowArch {
		fmt.Printf("available archs:\n\t%s\n", strings.Join(osarches, "\n\t"))
		return
	}

	if *flagBinary == "" {
		log.Fatal("-binary required")
	}

	os.RemoveAll(*flagBuildDir)
	_ = os.MkdirAll(*flagBuildDir, os.ModePerm)
	compile()
}
