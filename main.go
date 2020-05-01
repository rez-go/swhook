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
)

func main() {
	flag.StringVar(&stackName, "stack", "", "help message for flagname")

	flag.Parse()

	if stackName == "" {
		fmt.Printf("The option --stack is required\n")
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
			fmt.Printf("Starting deploy for stack %q revision %s ...\n", stackName, eventData.After)
			repoURL := eventData.Repository.SSHURL
			action := NewStackDeploymentAction(stackName, repoURL, eventData.After)
			err = action.Run()
			if err != nil {
				panic(err)
			}
			fmt.Printf("Deploy completed for stack %q revision %s\n", stackName, eventData.After)
		}

		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":2002", nil)
}

func NewStackDeploymentAction(
	stackName string,
	composeRepoURL string,
	composeRevision string,
) *StackDeploymentAction {
	return &StackDeploymentAction{
		stackName:       stackName,
		composeRepoURL:  composeRepoURL,
		composeRevision: composeRevision,
	}
}

type StackDeploymentAction struct {
	stackName       string
	composeRepoURL  string
	composeRevision string
	composeFilename string
	workDir         string
}

func (action *StackDeploymentAction) Run() error {
	var err error
	action.workDir, err = ioutil.TempDir("", "swhook_")
	if err != nil {
		return err
	}
	defer os.RemoveAll(action.workDir)

	err = action.pullRepo()
	if err != nil {
		return err
	}

	err = action.deploy()

	return err
}

func (action *StackDeploymentAction) pullRepo() error {
	cmdEnv := os.Environ()[:]
	cmd := exec.Command("git", "clone", action.composeRepoURL, action.workDir)
	cmd.Env = cmdEnv
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, outBytes)
	}
	cmd = exec.Command("git", "reset", "--hard", action.composeRevision)
	cmd.Env = cmdEnv
	cmd.Dir = action.workDir
	outBytes, err = cmd.CombinedOutput()
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
	cmd := exec.Command("docker", "stack", "deploy", "--prune", "-c", composeFilename, stackName)
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
