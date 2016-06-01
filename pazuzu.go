package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
)

// Pazuzu defines pazuzu config.
type Pazuzu struct {
	registry       string
	dockerfile     []byte
	testSpec       string
	dockerEndpoint string
	docker         *docker.Client
}

// Feature defines a feature fetched from pazuzu-registry.
type Feature struct {
	Name            string
	DockerData      string `json:"docker_data"`
	TestInstruction string `json:"test_instruction"`
}

// Generate generates Dockfiler and test.spec file base on list of features
func (p *Pazuzu) Generate(features []string) error {
	fs, err := p.getFeatures(features)
	if err != nil {
		return err
	}

	err = p.generateDockerfile(fs)
	if err != nil {
		return err
	}

	err = p.generateTestSpec(fs)
	if err != nil {
		return err
	}

	return nil
}

// generate in-memory Dockerfile from list of features.
func (p *Pazuzu) generateDockerfile(features []Feature) error {
	var buf bytes.Buffer

	_, err := buf.WriteString("FROM ubuntu:latest\n")
	if err != nil {
		return err
	}

	for _, feature := range features {
		_, err = buf.WriteString(fmt.Sprintf("# %s\n", feature.Name))
		if err != nil {
			return err
		}

		_, err = buf.WriteString(fmt.Sprintf("%s\n", feature.DockerData))
		if err != nil {
			return err
		}
	}

	_, err = buf.WriteString("CMD /bin/bash\n")
	if err != nil {
		return err
	}

	p.dockerfile = buf.Bytes()

	return nil
}

type TestSpec struct {
	Feature string `json:"feature"`
	Cmd     string `json:"cmd"`
}

// generate test spec from list of features.
func (p *Pazuzu) generateTestSpec(features []Feature) error {
	f, err := os.Create(p.testSpec)
	if err != nil {
		return err
	}
	defer f.Close()

	var specs []TestSpec

	for _, feature := range features {
		spec := TestSpec{
			Feature: feature.Name,
			Cmd:     feature.TestInstruction,
		}

		specs = append(specs, spec)
	}

	enc := json.NewEncoder(f)
	return enc.Encode(specs)
}

// read test specs from file.
func (p *Pazuzu) readTestSpec() ([]TestSpec, error) {
	f, err := os.Open(p.testSpec)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(f)

	var specs []TestSpec

	err = dec.Decode(&specs)
	if err != nil {
		return nil, err
	}

	return specs, nil
}

// APIError defines error response from pazuzu-registry.
type APIError struct {
	Code            string
	Message         string
	DetailedMessage string
}

// get a list of features given the feature names.
func (p *Pazuzu) getFeatures(features []string) ([]Feature, error) {
	resp, err := http.Get(fmt.Sprintf("%s/features?name=%s", p.registry,
		strings.Join(features, ",")))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp APIError

		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&errResp)
		if err != nil {
			return nil, err
		}

		return nil, fmt.Errorf(errResp.Message)
	}

	var res []Feature

	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// DockerBuild builds a docker image based on the generated Dockerfile.
func (p *Pazuzu) DockerBuild(name string) error {
	client, err := docker.NewClient(p.dockerEndpoint)
	if err != nil {
		return err
	}

	t := time.Now()
	inputBuf := bytes.NewBuffer(nil)
	tr := tar.NewWriter(inputBuf)
	tr.WriteHeader(&tar.Header{
		Name:       "Dockerfile",
		Size:       int64(len(p.dockerfile)),
		ModTime:    t,
		AccessTime: t,
		ChangeTime: t,
	})
	tr.Write(p.dockerfile)
	tr.Close()

	opts := docker.BuildImageOptions{
		Name:         name,
		InputStream:  inputBuf,
		OutputStream: os.Stdout,
	}

	err = client.BuildImage(opts)
	if err != nil {
		return err
	}

	return nil
}

// Start a docker container running /bin/bash.
func (p *Pazuzu) dockerStart(image string) (*docker.Container, error) {
	var err error
	p.docker, err = docker.NewClient(p.dockerEndpoint)
	if err != nil {
		return nil, err
	}

	opts := docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: image,
			Tty:   true,
			Cmd: []string{
				"/bin/bash",
			},
		},
	}

	container, err := p.docker.CreateContainer(opts)
	if err != nil {
		return nil, err
	}

	err = p.docker.StartContainer(container.ID, nil)
	if err != nil {
		return nil, err
	}

	return container, nil
}

// Execute command in docker container.
// The command will run in /bin/bash -c ''.
func (p *Pazuzu) dockerExec(ID string, cmd string) error {
	execOpts := docker.CreateExecOptions{
		Container:    ID,
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Cmd: []string{
			"/bin/bash",
			"-c",
			cmd,
		},
		Tty: false,
	}

	exec, err := p.docker.CreateExec(execOpts)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	var errBuf bytes.Buffer

	startExecOpts := docker.StartExecOptions{
		Detach:       false,
		OutputStream: &buf,
		ErrorStream:  &errBuf,
		RawTerminal:  false,
		Tty:          false,
	}

	err = p.docker.StartExec(exec.ID, startExecOpts)
	if err != nil {
		return err
	}

	inspect, err := p.docker.InspectExec(exec.ID)
	if err != nil {
		return err
	}

	if inspect.ExitCode > 0 {
		return fmt.Errorf("exit code %d: %s", inspect.ExitCode, buf.String())
	}

	return nil
}

// Stop docker container by ID
func (p *Pazuzu) dockerStop(ID string) error {
	err := p.docker.StopContainer(ID, 1)
	if err != nil {
		return err
	}

	rmOpts := docker.RemoveContainerOptions{
		ID: ID,
	}

	err = p.docker.RemoveContainer(rmOpts)
	if err != nil {
		return err
	}

	return nil
}

// RunTestSpec runs the tests against the given image.
func (p *Pazuzu) RunTestSpec(image string) error {
	specs, err := p.readTestSpec()
	if err != nil {
		return err
	}

	container, err := p.dockerStart(image)
	if err != nil {
		return err
	}

	failedTests := 0

	for _, spec := range specs {
		fmt.Printf("Running test spec for feature '%s':\n\t%s\n",
			spec.Feature, spec.Cmd)
		err = p.dockerExec(container.ID, spec.Cmd)
		if err != nil {
			fmt.Println(err)
			failedTests++
		}
	}

	err = p.dockerStop(container.ID)
	if err != nil {
		return err
	}

	if failedTests > 0 {
		return fmt.Errorf("number of failing tests: %d", failedTests)
	}

	return nil
}
