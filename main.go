package main

import (
	"errors"
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

func main() {
	stackName := ""
	workDir := ""
	listenHostPort := ":8080"
	servePath := "/stack-deployments"
	secret := ""

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

	svc, err := NewStackDeploymentService(stackName, workDir, secret)
	if err != nil {
		log.Fatalf("Unable to initialize service: %v", err)
	}

	http.HandleFunc(servePath, svc.postStackDeployments)

	http.ListenAndServe(listenHostPort, nil)
}

// NewStackDeploymentService returns a new instance of StackDeploymentService.
func NewStackDeploymentService(
	stackName string,
	workDir string,
	secret string,
) (*StackDeploymentService, error) {
	if stackName == "" {
		return nil, errors.New("stack name must not be empty")
	}

	opts := []github.Option{}
	if secret != "" {
		opts = append(opts, github.Options.Secret(secret))
	}

	ghHook, err := github.New(opts...)
	if err != nil {
		log.Fatalf("GitHub hook handler instantiation: %v", err)
	}

	return &StackDeploymentService{
		githubHook: ghHook,
		stackName:  stackName,
	}, nil
}

// StackDeploymentService represents a service for a stack deployment.
type StackDeploymentService struct {
	githubHook *github.Webhook
	stackName  string
	workDir    string
}

func (svc *StackDeploymentService) postStackDeployments(w http.ResponseWriter, r *http.Request) {
	payload, err := svc.githubHook.Parse(r, github.PushEvent)
	if err != nil {
		if err == github.ErrEventNotFound {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	switch eventData := payload.(type) {
	case github.PushPayload:
		revision := eventData.After
		repoURL := eventData.Repository.SSHURL
		err := svc.updateDeployment(repoURL, revision)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
}

func (svc *StackDeploymentService) updateDeployment(repoURL string, revision string) error {
	log.Printf("stack %q rev %s: Preparing new deployment...",
		svc.stackName, revision[:12])
	action := NewStackDeploymentAction(svc.stackName, svc.workDir, repoURL, revision)
	go func() {
		defer func() {
			if recData := recover(); recData != nil {
				log.Printf("stack %q rev %s: Deployment update caused panic: %v",
					svc.stackName, revision[:12], recData)
			}
		}()
		err := action.Run()
		if err != nil {
			log.Printf("stack %q rev %s: Deployment update failed: %v",
				svc.stackName, revision[:12], err)
			return
		}
		log.Printf("stack %q rev %s: Deployment update complete.",
			svc.stackName, revision[:12])
	}()
	return nil
}

// NewStackDeploymentAction creates an action for updating the deployment.
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

// StackDeploymentAction holds information required for an update.
type StackDeploymentAction struct {
	stackName        string
	workDir          string
	workDirSpecified bool
	composeRepoURL   string
	composeRevision  string
	composeFilename  string
}

// Run runs the action.
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
