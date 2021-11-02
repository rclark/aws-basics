package configuration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Builds is the set of builds configured for a repository.
type Builds struct {
	DockerImages  DockerImageConfigs  `yaml:"docker-images,omitempty"`
	LambdaBundles LambdaBundleConfigs `yaml:"lambda-bundles,omitempty"`
}

func (b *Builds) serialize() ([]byte, error) {
	yml, err := yaml.Marshal(b)
	if err != nil {
		return nil, errors.Wrap(err, "serialization failure")
	}
	return yml, nil
}

func (b *Builds) deserialize(yml []byte) error {
	return errors.Wrap(yaml.Unmarshal(yml, b), "deserialzation failure")
}

// Read parses the builds.yaml file in the specified directory. If the file does
// not exist, a nil pointer is the first returned value, and there is no error.
func Read(dir string) (builds *Builds, err error) {
	builds = new(Builds)

	yml, err := os.ReadFile(filepath.Join(dir, "builds.yaml"))
	if err != nil {
		return nil, nil
	}

	err = builds.deserialize(yml)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse builds.yaml")
	}

	return
}

// Write serializes the configuration as YAML and writes it to builds.yaml in
// the specified directory.
func Write(dir string, builds *Builds) error {
	yml, err := builds.serialize()
	if err != nil {
		return errors.Wrap(err, "failed to generate yaml")
	}
	return errors.Wrap(
		os.WriteFile(filepath.Join(dir, "builds.yaml"), yml, 0666),
		"failed to write builds.yaml",
	)
}

// AddOrReplace prompts the user to either add updates to an existing
// builds.yaml file, or replace it. If there is no existing builds.yaml file in
// the specified directory, the new file is written without prompting.
func AddOrReplace(dir string, updates *Builds) error {
	existing, err := Read(dir)
	if err != nil {
		return errors.Wrap(err, "failed to read existing builds.yaml")
	}
	if existing == nil {
		return errors.Wrap(Write(dir, updates), "failed to write builds.yaml")
	}

	e, err := existing.serialize()
	if err != nil {
		return errors.Wrap(err, "failed to parse existing builds.yaml")
	}
	fmt.Println()
	fmt.Printf("%s\n", "\033[1m\033[32m*\033[0m \033[1mDetected existing builds.yaml file:\033[0m")
	fmt.Println()
	fmt.Println(string(e))

	var result string
	err = survey.AskOne(&survey.Select{
		Message: "Would you like to:",
		Options: []string{"Append your new build to this file?", "Overwrite this file with your new configuration?"},
	}, &result)
	if err != nil {
		return errors.Wrap(err, "prompting failure")
	}

	if result == "Overwrite this file with your new configuration?" {
		return errors.Wrap(Write(dir, updates), "failed to write builds.yaml")
	}

	existing.DockerImages = append(existing.DockerImages, updates.DockerImages...)
	existing.LambdaBundles = append(existing.LambdaBundles, updates.LambdaBundles...)
	return errors.Wrap(Write(dir, existing), "failed to write builds.yaml")
}

type prompts interface {
	Prompt() error
}

// Prompt walks the user through a series of terminal prompts to generate a new
// build configuration.
func (b *Builds) Prompt() error {
	prompt := &survey.Select{
		Message: "Which type of build would you like to add?",
		Options: []string{"Docker image", "Lambda bundle"},
	}

	var buildType string
	if err := survey.AskOne(prompt, &buildType); err != nil {
		return errors.Wrap(err, "prompting failure")
	}

	var next prompts
	switch buildType {
	case "Docker image":
		next = DefaultDockerImageConfig()
	case "Lambda bundle":
		next = DefaultLambdaBundleConfig()
	}

	if err := next.Prompt(); err != nil {
		return errors.Wrap(err, "configuration failure")
	}

	switch buildType {
	case "Docker image":
		b.DockerImages = append(b.DockerImages, next.(*DockerImageConfig))
	case "Lambda bundle":
		b.LambdaBundles = append(b.LambdaBundles, next.(*LambdaBundleConfig))
	}

	return nil
}

// DockerImageConfigs are a set of configurations for Docker image builds.
type DockerImageConfigs []*DockerImageConfig

// DockerImageConfig are configuration settings for a Docker image build.
type DockerImageConfig struct {
	DockerfilePath string   `yaml:"dockerfile"`
	Context        string   `yaml:"context"`
	Triggers       Triggers `yaml:"triggers"`
}

// Prompt walks the user through a series of terminal prompts to generate a new
// Docker image build configuration.
func (d *DockerImageConfig) Prompt() error {
	prompts := []*survey.Question{
		{
			Name: "DockerfilePath",
			Prompt: &survey.Input{
				Message: "Path to Dockerfile:",
				Default: d.DockerfilePath,
			},
			Validate: func(ans interface{}) error {
				result := ans.(string)
				abs, err := filepath.Abs(result)
				if err != nil {
					return errors.Wrap(err, "invalid file path")
				}

				info, err := os.Stat(abs)
				if err != nil {
					return errors.Wrap(err, fmt.Sprintf("could not find file %s", abs))
				}

				if info.IsDir() {
					return errors.New(fmt.Sprintf("%s is not a file", abs))
				}

				return nil
			},
		},
		{
			Name: "Context",
			Prompt: &survey.Input{
				Message: "Working directory for the build:",
				Default: d.Context,
			},
			Validate: func(ans interface{}) error {
				result := ans.(string)
				abs, err := filepath.Abs(result)
				if err != nil {
					return errors.Wrap(err, "invalid path")
				}

				info, err := os.Stat(abs)
				if err != nil {
					return errors.Wrap(err, fmt.Sprintf("could not find %s", abs))
				}

				if !info.IsDir() {
					return errors.New(fmt.Sprintf("%s is a file, not a directory", abs))
				}

				return nil
			},
		},
	}

	err := survey.Ask(prompts, d)
	if err != nil {
		return errors.Wrap(err, "prompting failure")
	}

	return errors.Wrap(d.Triggers.Prompt(), "configuration failure")
}

// DefaultDockerImageConfig creates a new DockerImageConfig with a default set
// of configurations.
func DefaultDockerImageConfig() *DockerImageConfig {
	return &DockerImageConfig{
		DockerfilePath: "Dockerfile",
		Context:        ".",
		Triggers:       DefaultTriggers(),
	}
}

// LambdaBundleConfigs are a set of configurations for Lambda bundle builds.
type LambdaBundleConfigs []*LambdaBundleConfig

// LambdaBundleConfig are configuration settings for a Lambda bundle build.
type LambdaBundleConfig struct {
	Runtime      string   `yaml:"runtime"`
	BuildCommand string   `yaml:"cmd,omitempty"`
	IncludePaths []string `yaml:"includes,omitempty"`
	ExcludePaths []string `yaml:"excludes,omitempty"`
	Triggers     Triggers `yaml:"triggers"`
}

var runtimes = []string{"go1.x", "nodejs14.x"}

// Prompt walks the user through a series of terminal prompts to generate a new
// Lambda bundle build configuration.
func (l *LambdaBundleConfig) Prompt() error {
	runtime := []*survey.Question{
		{
			Name: "Runtime",
			Prompt: &survey.Input{
				Message: "Lambda runtime environment:",
				Default: l.Runtime,
				Suggest: func(toComplete string) (suggest []string) {
					for _, rt := range runtimes {
						if strings.HasPrefix(rt, toComplete) {
							suggest = append(suggest, rt)
						}
					}
					return
				},
			},
			Validate: func(ans interface{}) error {
				result := ans.(string)
				for _, rt := range runtimes {
					if rt == result {
						return nil
					}
				}
				return errors.New(fmt.Sprintf("runtime must be one of %s", strings.Join(runtimes, ", ")))
			},
		},
	}

	err := survey.Ask(runtime, l)
	if err != nil {
		return errors.Wrap(err, "prompting failure")
	}

	defaultCmd := ""
	switch l.Runtime {
	case "go1.x":
		defaultCmd = "make build"
	case "nodejs14.x":
		defaultCmd = "npm ci"
	}

	cmd := []*survey.Question{
		{
			Name: "BuildCommand",
			Prompt: &survey.Input{
				Message: "Customize the build command:",
				Default: defaultCmd,
			},
		},
	}

	err = survey.Ask(cmd, l)
	if err != nil {
		return errors.Wrap(err, "prompting failure")
	}

	if l.Runtime == "nodejs14.x" {
		setPaths := ""
		err = survey.AskOne(&survey.Select{
			Renderer: survey.Renderer{},
			Message:  "Are there files in your repository you'd like to specifically include or exclude from the bundle?",
			Options:  []string{"Use all files", "Include specific files", "Exclude certain files"},
			Default:  "Use all files",
		}, &setPaths)

		switch setPaths {
		case "Include specific files":
			survey.Ask([]*survey.Question{{
				Name:   "IncludePaths",
				Prompt: &survey.Input{Message: "Paths to include, comma-delimited:"},
				Transform: func(ans interface{}) interface{} {
					result := ans.(string)
					split := strings.Split(result, ",")
					for i, s := range split {
						split[i] = strings.Trim(s, " ")
					}
					return split
				},
			}}, l)
		case "Exclude certain files":
			survey.Ask([]*survey.Question{{
				Name:   "ExcludePaths",
				Prompt: &survey.Input{Message: "Paths to exclude, comma-delimited:"},
				Transform: func(ans interface{}) interface{} {
					result := ans.(string)
					split := strings.Split(result, ",")
					for i, s := range split {
						split[i] = strings.Trim(s, " ")
					}
					return split
				},
			}}, l)
		}
	}

	return errors.Wrap(l.Triggers.Prompt(), "configuration failure")
}

// DefaultLambdaBundleConfig creates a new LambdaBundleConfig with a default set
// of configurations.
func DefaultLambdaBundleConfig() *LambdaBundleConfig {
	return &LambdaBundleConfig{
		Runtime:      "go1.x",
		BuildCommand: "make build",
		IncludePaths: []string{},
		ExcludePaths: []string{},
		Triggers:     DefaultTriggers(),
	}
}

// Triggers are a set of descriptions for when commits should result in builds.
type Triggers struct {
	Branches []string `yaml:"branches,omitempty"`
	Keywords []string `yaml:"keywords,omitempty"`
}

// Prompt walks the user through a series of terminal prompts to add new build
// triggers.
func (t *Triggers) Prompt() error {
	var useDefaults bool
	def := &survey.Confirm{
		Message: "Would you like to setup custom build triggers?",
		Default: false,
	}

	err := survey.AskOne(def, &useDefaults)
	if err != nil {
		return errors.Wrap(err, "prompting failure")
	}

	if !useDefaults {
		*t = DefaultTriggers()
		return nil
	}

	*t = Triggers{}

	types := &survey.MultiSelect{
		Message: "Select the types of build triggers you would like to configure:",
		Options: []string{"Commit message keywords", "Branch names"},
	}

	var triggerTypes []string
	err = survey.AskOne(types, &triggerTypes)
	if err != nil {
		return errors.Wrap(err, "prompting failure")
	}

	for _, tt := range triggerTypes {
		switch tt {
		case "Commit message keywords":
			keywords := &survey.Input{
				Message: "Keywords in commit messages to trigger builds, comma-delimited:",
				Default: strings.Join(t.Keywords, ", "),
			}

			var result string
			err = survey.AskOne(keywords, &result)
			if err != nil {
				if err == terminal.InterruptErr {
					return nil
				}
				return errors.Wrap(err, "prompting failure")
			}

			names := strings.Split(result, ",")
			for i, b := range names {
				names[i] = strings.Trim(b, " ")
			}
			t.Keywords = names

		case "Branch names":
			branches := &survey.Input{
				Message: "Branch names to build on every commit, comma-delimited:",
				Default: strings.Join(t.Branches, ", "),
			}

			var result string
			err = survey.AskOne(branches, &result)
			if err != nil {
				if err == terminal.InterruptErr {
					return nil
				}
				return errors.Wrap(err, "prompting failure")
			}

			names := strings.Split(result, ",")
			for i, b := range names {
				names[i] = strings.Trim(b, " ")
			}
			t.Branches = names
		}
	}

	return nil
}

// DefaultTriggers returns the default build trigger. This triggers builds when
// commit messages include [build].
func DefaultTriggers() Triggers {
	return Triggers{
		Keywords: []string{"[build]"},
	}
}
