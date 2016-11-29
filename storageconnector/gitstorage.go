package storageconnector

import (
	"io/ioutil"
	"path"
	"regexp"
	"strings"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/core"
	"gopkg.in/yaml.v2"

	"github.com/zalando-incubator/pazuzu/shared"
)

const (
	featureDir               = "features"   // name of the directory where features are located.
	featureFile              = "meta.yml"   // name of the file containing all metadata for a feature.
	featureSnippet           = "Dockerfile" // the file containing the actual docker snippet.
	testSnippet              = "test.bats"  // the file containing the bats test specification (https://github.com/sstephenson/bats)
	defaultSearchParamsLimit = 100 // we should use this constant
)

// yamlFeatureMeta is used for unmarshalling of meta.yml files.
type yamlFeatureMeta struct {
	Description  string
	Author       string
	Dependencies []string
}

// GitStorage is an implementation of StorageReader based on
// a git repository as storage back-end.
type GitStorage struct {
	repo *git.Repository
}

// NewGitStorage returns a StorageReader which uses a public git repository
// as data source for pazuzu features.
//
// url:  The URL to the git repository that serves as data source. The
//       repository must be publicly accessible.
//
// If the repository can't be accessed NewStorageReader returns an error.
func NewGitStorage(url string) (*GitStorage, error) {
	// OPTIMIZATION: can be an fs repository which is cached and only pulled when needed
	repo := git.NewMemoryRepository()

	err := repo.Clone(&git.CloneOptions{
		URL:           url,
		ReferenceName: core.HEAD,
		SingleBranch:  true,
		Depth:         1,
	})
	if err != nil {
		return nil, err
	}

	return &GitStorage{repo: repo}, nil
}

func (storage *GitStorage) SearchMeta(name *regexp.Regexp) ([]shared.FeatureMeta, error) {
	commit, err := storage.latestCommit()
	if err != nil {
		return nil, err
	}

	all, err := commit.Files()
	if err != nil {
		return nil, err
	}

	// find matching feature names
	matchedNames := map[string]bool{}
	matchedFeatures := []shared.FeatureMeta{}
	err = all.ForEach(func(file *git.File) error {
		pathComponents := strings.Split(file.Name, "/")

		// check if file is in feature dir
		if pathComponents[0] != featureDir {
			return nil
		}

		// check if feature was already found
		featureName := pathComponents[1]
		if matchedNames[featureName] {
			return nil
		}

		// check if feature matches search params
		if name.MatchString(featureName) {
			meta, err := getMeta(commit, featureName)
			if err != nil {
				return err
			}
			matchedFeatures = append(matchedFeatures, meta)
			matchedNames[featureName] = true
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return matchedFeatures, nil
}

func (storage *GitStorage) GetMeta(name string) (shared.FeatureMeta, error) {
	commit, err := storage.latestCommit()
	if err != nil {
		return shared.FeatureMeta{}, err
	}

	return getMeta(commit, name)
}

// getMeta returns metadata about a feature from a given commit.
//
// commit:  The commit from which to obtain the feature information.
// name:    The exact feature name.
func getMeta(commit *git.Commit, name string) (shared.FeatureMeta, error) {
	file, err := commit.File(path.Join(featureDir, name, featureFile))

	if err != nil {
		return shared.FeatureMeta{}, err
	}

	reader, err := file.Reader()
	if err != nil {
		return shared.FeatureMeta{}, err
	}

	content, err := ioutil.ReadAll(reader)
	if err != nil {
		return shared.FeatureMeta{}, err
	}

	meta := &yamlFeatureMeta{}
	err = yaml.Unmarshal(content, meta)
	if err != nil {
		return shared.FeatureMeta{}, err
	}

	return shared.FeatureMeta{
		Name:         name,
		Dependencies: meta.Dependencies,
		Description:  meta.Description,
		Author:       meta.Author,
		UpdatedAt:    commit.Committer.When,
	}, nil
}

func (storage *GitStorage) GetFeature(name string) (shared.Feature, error) {
	commit, err := storage.latestCommit()
	if err != nil {
		return shared.Feature{}, err
	}

	return getFeature(commit, name)
}

// getFeature returns all data of a feature from a given commit.
//
// commit:  The commit from which to obtain the feature information.
// name:    The exact feature name.
func getFeature(commit *git.Commit, name string) (shared.Feature, error) {
	meta, err := getMeta(commit, name)
	if err != nil {
		return shared.Feature{}, err
	}

	file, err := commit.File(path.Join(featureDir, name, featureSnippet))
	if err != nil {
		if err == git.ErrFileNotFound {
			return shared.Feature{Meta: meta}, nil
		}
		return shared.Feature{}, err
	}

	reader, err := file.Reader()
	if err != nil {
		return shared.Feature{}, err
	}

	snippet, err := ioutil.ReadAll(reader)
	if err != nil {
		return shared.Feature{}, err
	}

	testSnippet := getTestSpec(commit, name)

	return shared.Feature{
		Meta:         meta,
		Snippet:      string(snippet),
		TestSnippet:  string(testSnippet),
	}, nil
}

// getTestSpec returns test.bats file content
//
// commit:  The commit from which to obtain the feature information.
// name:    The exact feature name.
func getTestSpec(commit *git.Commit, name string) string {
	file, err := commit.File(path.Join(featureDir, name, testSnippet))
	if err != nil {
		return ""
	}

	reader, err := file.Reader()
	if err != nil {
		return ""
	}

	content, err := shared.ReadTestSpec(reader)
	if err != nil {
		return ""
	}

	return content
}

func (storage *GitStorage) Resolve(names ...string) ([]string, map[string]shared.Feature, error) {
	var slice []string

	commit, err := storage.latestCommit()
	if err != nil {
		return []string {}, map[string]shared.Feature{}, err
	}

	result := map[string]shared.Feature{}
	for _, name := range names {
		err = resolve(commit, name, &slice, result)
		if err != nil {
			return []string {}, map[string]shared.Feature{}, err
		}
	}

	return slice, result, nil
}

// resolve returns all data for a certain feature and its direct and indirect
// dependencies. All feature data is added to the provided result map.
//
// commit:  The commit from which to obtain the feature information.
// name:    The exact feature name.
// result:  All features collected so far.
func resolve(commit *git.Commit, name string, list *[]string, result map[string]shared.Feature) error {
	if _, ok := result[name]; ok {
		return nil
	}

	feature, err := getFeature(commit, name)
	if err != nil {
		return err
	}

	for _, depName := range feature.Meta.Dependencies {
		err = resolve(commit, depName, list, result)
		if err != nil {
			return err
		}
	}
	*list = append(*list, name)
	result[name] = feature

	return nil
}

// latestCommit is a helper method which gets the latest commit (HEAD) from
// a the storage git repository.
func (storage *GitStorage) latestCommit() (*git.Commit, error) {
	head, err := storage.repo.Head()
	if err != nil {
		return nil, err
	}

	commit, err := storage.repo.Commit(head.Hash())
	if err != nil {
		return nil, err
	}

	return commit, nil
}
