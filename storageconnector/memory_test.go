package storageconnector

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"regexp"
)

func TestMemoryGet(t *testing.T) {
	storage := NewMemory([]Feature{
		{
			Meta: FeatureMeta{
				Name:         "FeatureA",
				Author:       "SomeAuthor",
				Dependencies: []string{"FeatureB", "FeatureC"},
			},
			Snippet: "",
		},
		{
			Meta: FeatureMeta{
				Name:   "FeatureB",
				Author: "SomeAuthor",
			},
			Snippet: "",
		},
	})

	t.Run("Run search and find 1 Feature", func(t *testing.T) {
		result, err := storage.SearchMeta(SearchParams{
			Name: regexp.MustCompile("FeatureA*"),
		})

		assert.Nil(t, err)
		assert.Equal(t, []FeatureMeta{{
			Name:         "FeatureA",
			Author:       "SomeAuthor",
			Dependencies: []string{"FeatureB", "FeatureC"},
		}}, result)
	})

	t.Run("Run search and find no Features", func(t *testing.T) {
		result, err := storage.SearchMeta(SearchParams{
			Name: regexp.MustCompile("FooBoo"),
		})

		assert.Nil(t, err)
		assert.Equal(t, []FeatureMeta{}, result)
	})

	t.Run("Try to get a feature Meta and find it", func(t *testing.T) {
		result, err := storage.GetMeta("FeatureA")

		assert.Nil(t, err)
		assert.Equal(t, FeatureMeta{
			Name:         "FeatureA",
			Author:       "SomeAuthor",
			Dependencies: []string{"FeatureB", "FeatureC"},
		}, result)
	})

	t.Run("Try to get a feature Meta and find nothing", func(t *testing.T) {
		_, err := storage.GetMeta("FooBoo")
		assert.NotNil(t, err)
		assert.Equal(t, "Feature 'FooBoo' was not found", err.Error())
	})

	t.Run("Try to get a feature and find it", func(t *testing.T) {
		result, err := storage.GetFeature("FeatureA")

		assert.Nil(t, err)
		assert.Equal(t, Feature{
			{
				Meta: FeatureMeta{
					Name:         "FeatureA",
					Author:       "SomeAuthor",
					Dependencies: []string{"FeatureB", "FeatureC"},
				},
				Snippet: "",
			},
		}, result)
	})

	t.Run("Try to get a feature and find nothing", func(t *testing.T) {
		_, err := storage.GetFeature("FooBoo")
		assert.NotNil(t, err)
		assert.Equal(t, "Feature 'FooBoo' was not found", err.Error())
	})
}

func TestDummyResolve(t *testing.T) {
	storage := NewMemory([]Feature{
		{
			Meta: FeatureMeta{
				Name:         "FeatureA",
				Author:       "SomeAuthor",
				Dependencies: []string{"FeatureB", "FeatureC"},
			},
			Snippet: "",
		},
		{
			Meta: FeatureMeta{
				Name:   "FeatureB",
				Author: "SomeAuthor",
			},
			Snippet: "",
		},
		{
			Meta: FeatureMeta{
				Name:         "FeatureC",
				Author:       "SomeAuthor",
				Dependencies: []string{"FeatureD"},
			},
			Snippet: "",
		},
		{
			Meta: FeatureMeta{
				Name:   "FeatureD",
				Author: "SomeAuthor",
			},
			Snippet: "",
		},
	})

	t.Run("Resolve FeatureA", func(t *testing.T) {
		expected := map[string]Feature{
			"FeatureA": {
				Meta: FeatureMeta{
					Name:         "FeatureA",
					Author:       "SomeAuthor",
					Dependencies: []string{"FeatureB", "FeatureC"},
				},
				Snippet: "",
			},
			"FeatureB":                        {
				Meta: FeatureMeta{
					Name:   "FeatureB",
					Author: "SomeAuthor",
				},
				Snippet: "",
			},
			"FeatureC":                        {
				Meta: FeatureMeta{
					Name:         "FeatureC",
					Author:       "SomeAuthor",
					Dependencies: []string{"FeatureD"},
				},
				Snippet: "",
			},
			"FeatureD":                        {
				Meta: FeatureMeta{
					Name:   "FeatureD",
					Author: "SomeAuthor",
				},
				Snippet: "",
			},
		}

		result, err := storage.Resolve("FeatureA")
		assert.Nil(t, err)
		assert.Equal(t, expected, result)
	})

	t.Run("Resolve FeatureB and FeatureD", func(t *testing.T) {
		expected := map[string]Feature{
			"FeatureB":                        {
				Meta: FeatureMeta{
					Name:   "FeatureB",
					Author: "SomeAuthor",
				},
				Snippet: "",
			},
			"FeatureC":                        {
				Meta: FeatureMeta{
					Name:         "FeatureC",
					Author:       "SomeAuthor",
					Dependencies: []string{"FeatureD"},
				},
				Snippet: "",
			},
			"FeatureD":                        {
				Meta: FeatureMeta{
					Name:   "FeatureD",
					Author: "SomeAuthor",
				},
				Snippet: "",
			},
		}

		result, err := storage.Resolve("FeatureB", "FeatureD")
		assert.Nil(t, err)
		assert.Equal(t, expected, result)
	})

	t.Run("Resolve Non-existing feature", func(t *testing.T) {
		_, err := storage.Resolve("FooBoo", "FeatureD")
		assert.NotNil(t, err)
		assert.Equal(t, "Feature 'FooBoo' was not found", err.Error())
	})

	t.Run("Resolve empty list of features", func(t *testing.T) {
		_, err := storage.Resolve()
		assert.NotNil(t, err)
		assert.Equal(t, "No features provided to resolve", err.Error())
	})
}