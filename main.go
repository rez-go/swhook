package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	flag "github.com/spf13/pflag"
	"gopkg.in/go-playground/webhooks.v5/github"
)

const (
	servePath = "/stack-deployments"
)

var (
	stackName = ""
	workDir   = ""
)

func main() {
	flag.StringVar(&stackName, "stack", "", "The name of the stack.")
	flag.StringVar(&workDir, "workdir", "",
		"If provided, swhook will use this directory to checkout the repository. "+
			"By default, temporary directory will be used.")

	flag.Parse()

	if stackName == "" {
		fmt.Printf("Option --stack is required\n")
		os.Exit(-1)
	}

	hook, _ := github.New()

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
			fmt.Printf("Preparing new deployment for stack %q revision %s ...\n", stackName, eventData.After)
			repoURL := eventData.Repository.SSHURL
			revision := eventData.After
			action := NewStackDeploymentAction(stackName, workDir, repoURL, revision)
			go func() {
				err = action.Run()
				if err != nil {
					panic(err)
				}
				fmt.Printf("Deployment completed for stack %q revision %s\n", stackName, revision)
			}()
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
	})

	http.ListenAndServe(":2002", nil)
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

	if !action.workDirSpecified {
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
	fmt.Printf("Excuting deployment for stack %q revision %s with compose file %q ...\n",
		stackName, action.composeRevision, composeFilename)
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
