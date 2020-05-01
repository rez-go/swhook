package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	flag "github.com/spf13/pflag"
	"gopkg.in/go-playground/webhooks.v5/github"
)

const (
	servePathDefault = "/stack-deployments"
)

var (
	stackName      = ""
	workDir        = ""
	listenHostPort = ":8080"
	servePath      = servePathDefault
	secret         = ""
)

func main() {
	flag.StringVar(&stackName, "stack", "",
		"The name of the stack.")
	flag.StringVar(&workDir, "workdir", "",
		"If provided, swhook will use this directory to checkout the "+
			"repository. By default, temporary directory will be used.")
	flag.StringVar(&listenHostPort, "listen", listenHostPort,
		"Where should the service listen to.")
	flag.StringVar(&secret, "secret", "",
		"The secret used to validate the "+
			"requests comming to the hook.")

	flag.Parse()

	if stackName == "" {
		log.Printf("Option --stack is required")
		os.Exit(-1)
	}

	opts := []github.Option{}
	if secret != "" {
		opts = append(opts, github.Options.Secret(secret))
	}

	hook, err := github.New(opts...)
	if err != nil {
		log.Fatalf("GitHub hook handler instantiation: %v", err)
	}

	http.HandleFunc(servePath, func(w http.ResponseWriter, r *http.Request) {
		payload, err := hook.Parse(r, github.PushEvent)
		if err != nil {
			if err == github.ErrEventNotFound {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		switch eventData := payload.(type) {
		case github.PushPayload:
			revision := eventData.After
			log.Printf("stack %q rev %s: Preparing new deployment...",
				stackName, revision[:12])
			repoURL := eventData.Repository.SSHURL
			action := NewStackDeploymentAction(stackName, workDir, repoURL, revision)
			go func() {
				err = action.Run()
				if err != nil {
					panic(err)
				}
				log.Printf("stack %q rev %s: Deployment update complete.",
					stackName, revision[:12])
			}()
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
	})

	http.ListenAndServe(listenHostPort, nil)
}

func NewStackDeploymentAction(
	stackName string,
	workDir string,
	composeRepoURL string,
	composeRevision string,
) *StackDeploymentAction {
	return &StackDeploymentAction{
		stackName:        stackName,
		workDir:          workDir,
		workDirSpecified: workDir != "",
		composeRepoURL:   composeRepoURL,
		composeRevision:  composeRevision,
	}
}

type StackDeploymentAction struct {
	stackName        string
	workDir          string
	workDirSpecified bool
	composeRepoURL   string
	composeRevision  string
	composeFilename  string
}

func (action *StackDeploymentAction) Run() error {
	var err error
	if !action.workDirSpecified {
		action.workDir, err = ioutil.TempDir("", "swhook_")
		if err != nil {
			return err
		}
		defer os.RemoveAll(action.workDir)
	}

	err = action.pull()
	if err != nil {
		return err
	}

	err = action.deploy()

	return err
}

func (action *StackDeploymentAction) pull() error {
	cmdEnv := os.Environ()[:]

	workDirExists := false
	if action.workDirSpecified {
		workDirExists = dirExists(filepath.Join(action.workDir, ".git"))
	}

	if !workDirExists {
		cmd := exec.Command("git", "clone", action.composeRepoURL, action.workDir)
		cmd.Env = cmdEnv
		outBytes, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git clone: %w\n%s", err, outBytes)
		}
	} else {
		cmd := exec.Command("git", "pull")
		cmd.Env = cmdEnv
		cmd.Dir = action.workDir
		outBytes, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git pull: %w\n%s", err, outBytes)
		}
	}

	cmd := exec.Command("git", "reset", "--hard", action.composeRevision)
	cmd.Env = cmdEnv
	cmd.Dir = action.workDir
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset: %w\n%s", err, outBytes)
	}
	return nil
}

func (action *StackDeploymentAction) deploy() error {
	stackName := action.stackName
	composeFilename := action.composeFilename
	if composeFilename == "" {
		suffixes := []string{".yml", ".yaml", "-stack.yml", "-stack.yaml"}
		for _, sfx := range suffixes {
			fname := stackName + sfx
			if fileExists(fname) {
				composeFilename = fname
				break
			}
		}
		if composeFilename == "" {
			composeFilename = "main-stack.yaml"
		}
	}

	log.Printf("stack %q rev %s: Executing stack update with compose file %q ...",
		stackName, action.composeRevision[:12], composeFilename)
	cmd := exec.Command("docker", "stack", "deploy",
		"--prune", "-c", composeFilename, stackName)
	cmdEnv := os.Environ()[:]
	cmd.Env = cmdEnv
	cmd.Dir = action.workDir
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker stack deploy: %w\n%s", err, outBytes)
	}
	return nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func dirExists(dirname string) bool {
	info, err := os.Stat(dirname)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}
