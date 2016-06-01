package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli"
)

var version = "0.1"

var buildCmd = cli.Command{
	Name:   "build",
	Usage:  "build docker image",
	Action: buildFeatures,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "image-name, n",
			Value: "pazuzu-img",
			Usage: "Set docker image name",
		},
		cli.StringFlag{
			Name:  "test-spec, t",
			Value: "test_spec.json",
			Usage: "Set path to test spec file",
		},
		cli.BoolFlag{
			Name:  "verify",
			Usage: "Run test spec as part of the build",
		},
	},
}

var verifyCmd = cli.Command{
	Name:   "verify",
	Usage:  "verify docker image against",
	Action: verifyImage,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "test-spec, t",
			Value: "test_spec.json",
			Usage: "Set path to test spec file",
		},
	},
}

// Verifies the docker image produced by the build command against the test
// spec.
func verifyImage(c *cli.Context) error {
	pazuzu := Pazuzu{
		registry:       HttpRegistry("http://localhost:8080/api"),
		testSpec:       c.String("test-spec"),
		dockerEndpoint: c.GlobalString("docker-endpoint"),
	}

	err := pazuzu.RunTestSpec(c.Args().First())
	if err != nil {
		return err
	}

	return nil
}

// Fetches and builds features into a docker image.
func buildFeatures(c *cli.Context) error {
	pazuzu := Pazuzu{
		registry:       HttpRegistry("http://localhost:8080/api"),
		testSpec:       c.String("test-spec"),
		dockerEndpoint: c.GlobalString("docker-endpoint"),
	}

	if len(c.Args()) == 0 {
		return fmt.Errorf("no features specified")
	}

	err := pazuzu.Generate(c.Args())
	if err != nil {
		return err
	}

	err = pazuzu.DockerBuild(c.String("image-name"))
	if err != nil {
		return err
	}

	if c.Bool("verify") {
		err := pazuzu.RunTestSpec(c.String("image-name"))
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "pazuzu"
	app.Version = version
	app.Usage = "Build Docker features from pazuzu-registry"
	app.Commands = []cli.Command{
		buildCmd,
		verifyCmd,
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "docker-endpoint, e",
			Value: "unix:///var/run/docker.sock",
			Usage: "Set the docker endpoint",
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
