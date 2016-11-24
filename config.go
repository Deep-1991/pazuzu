package pazuzu

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/jinzhu/copier"
	"github.com/cevaris/ordered_map"
	"gopkg.in/yaml.v2"

	"github.com/zalando-incubator/pazuzu/storageconnector"
)

const (
	UserConfigFilenamePart = ".pazuzu-cli.yaml"

	// URL : default features-repo.
	URL = "https://github.com/zalando-incubator/pazuzu.git"
	// BaseImage : Base feature.
	BaseImage = "ubuntu:14.04"

	// StorageTypeGit : Git storage type.
	StorageTypeGit = "git"
	// StorageTypeMemory : Memory storage type.
	StorageTypeMemory = "memory"
)

var config Config

// GitConfig : config structure for Git-storage.
type GitConfig struct {
	URL string `yaml:"url" setter:"SetURL" help:"Git Repository URL."`
}

// MemoryConfig : config structure for Memory-storage.
type MemoryConfig struct {
	InitialiseRandom bool `yaml:"random_init" help:"???"`
	RandomSetSize    int  `yaml:"random_size" help:"???"`
}

// Config : actual config data structure.
type Config struct {
	Base        string       `yaml:"base" setter:"SetBase" help:"Base image name and tag (ex: 'ubuntu:14.04')"`
	StorageType string       `yaml:"storage" setter:"SetStorageType" help:"Storage-type ('git' or 'memory')"`
	Git         GitConfig    `yaml:"git" help:"Git storage configs."`
	Memory      MemoryConfig `yaml:"memory" help:"Memory storage configs."`
}

// SetBase : Setter of "Base".
func (c *Config) SetBase(base string) {
	c.Base = base
}

// SetStorageType : Setter of "StorageType".
func (c *Config) SetStorageType(storageType string) {
	c.StorageType = storageType
}

// SetGit : Setter of Git-Storage specific configuration.
func (c *Config) SetGit(git GitConfig) {
	c.Git = git
}

// SetURL : Setter of GitConfig.URL.
func (g *GitConfig) SetURL(url string) {
	g.URL = url
}

// InitDefaultConfig : Initialize config variable with defaults. (Does not loading configuration file)
func InitDefaultConfig() {
	config = Config{
		StorageType: "git",
		Base:        BaseImage,
		Git:         GitConfig{URL: URL},
		Memory: MemoryConfig{
			InitialiseRandom: false,
		},
	}
}

// NewConfig : Please call this function before GetConfig and only once in your application.
// Attempts load config file, but when it fails just use default configuration.
func NewConfig() error {
	InitDefaultConfig()
	config.Load()
	configMirror = config.InitConfigFieldMirrors()
	return nil
}

// GetConfig : get loaded config.
func GetConfig() *Config {
	return &config
}

// GetStorageReader : create new StorageReader by StorageType of given config.
func GetStorageReader(config Config) (storageconnector.StorageReader, error) {
	switch config.StorageType {
	case StorageTypeMemory:
		data := []storageconnector.Feature{}
		if config.Memory.InitialiseRandom {
			data = generateRandomFeatures(config.Memory.RandomSetSize)
		}

		return storageconnector.NewMemoryStorage(data), nil // implement a generator of random list of features?
	case StorageTypeGit:
		return storageconnector.NewGitStorage(config.Git.URL)
	}

	return nil, fmt.Errorf("unknown storage type '%s'", config.StorageType)
}

func generateRandomFeatures(setsize int) []storageconnector.Feature {
	// TODO: implement in case of need
	return []storageconnector.Feature{}
}

func UserHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func UserConfigFilename() string {
	return filepath.Join(UserHomeDir(), UserConfigFilenamePart)
}

func (c *Config) SaveToWriter(writer io.Writer) error {
	data, err := yaml.Marshal(c)
	_, err = writer.Write(data)
	return err
}

func LoadConfigFromReader(reader io.Reader) (Config, error) {
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		return Config{}, err
	}

	c := &Config{}
	err = yaml.Unmarshal(content, c)
	if err != nil {
		return Config{}, err
	}

	return *c, nil
}

func (c *Config) Load() {
	configFn := UserConfigFilename()
	c.LoadFromFile(configFn)
}

func (c *Config) LoadFromFile(configFn string) {
	f, err := os.Open(configFn)
	if err != nil {
		log.Printf("Cannot open config-file [%s], Reason = [%s], SKIP\n",
			configFn, err)
		return
	}
	defer f.Close()

	// replace cfg?
	cfg2, errLoad := LoadConfigFromReader(f)
	if errLoad != nil {
		log.Printf("Cannot load from [%s], Reason = [%s], SKIP\n",
			configFn, errLoad)
		return
	}

	errCopy := copier.Copy(c, &cfg2)
	if errCopy != nil {
		log.Printf("Cannot copy [%v] to [%v], Reason = [%s], SKIP\n",
			cfg2, c, errCopy)
		return
	}
}

func (c *Config) Save() error {
	configFn := UserConfigFilename()
	return c.SaveToFile(configFn)
}

func (c *Config) SaveToFile(configFn string) error {
	f, err := os.Create(configFn)
	if err != nil {
		return err
	}
	defer f.Close()

	errWriter := c.SaveToWriter(f)
	if errWriter != nil {
		return errWriter
	}
	return nil
}

type ConfigTraverseFunc func(field reflect.StructField,
	aVal reflect.Value, aType reflect.Type,
	addressableVal reflect.Value,
	ancestors []reflect.StructField) error

func (c *Config) TraverseEachField(cb ConfigTraverseFunc) error {
	aType := reflect.TypeOf(*c)
	aVal := reflect.ValueOf(*c)
	addressableVal := reflect.ValueOf(c)
	return traverseEachFieldRecur(aVal, aType, addressableVal, []reflect.StructField{}, cb)
}

func traverseEachFieldRecur(aVal reflect.Value, aType reflect.Type, addressableVal reflect.Value,
	ancestors []reflect.StructField, cb ConfigTraverseFunc) error {
	//
	for i := 0; i < aType.NumField(); i++ {
		field := aType.Field(i)
		if field.Type.Kind() == reflect.Struct {
			bType := field.Type
			f := reflect.Indirect(aVal).FieldByName(field.Name)
			f2 := reflect.Indirect(addressableVal).FieldByName(field.Name)
			err := traverseEachFieldRecur(f, bType, f2.Addr(), append(ancestors, field), cb)
			if err != nil {
				return err
			}
		} else {
			err := cb(field, aVal, aType, addressableVal, ancestors)
			if err != nil {
				return err
			}
		}
	}
	//
	return nil
}

type ConfigFieldMirror struct {
	Help   string
	Repr   string
	Setter reflect.Value
}

type ConfigMirror struct {
	M *ordered_map.OrderedMap
	C *Config
}

var configMirror *ConfigMirror

func (c *Config) InitConfigFieldMirrors() *ConfigMirror {
	m := ordered_map.NewOrderedMap()
	_ = c.TraverseEachField(func(field reflect.StructField,
		aVal reflect.Value, aType reflect.Type, addressableVal reflect.Value,
		ancestors []reflect.StructField) error {
		//
		configPath := makeConfigPathString(ancestors, field)
		tag := field.Tag
		help := ""
		repr := ""
		setter := reflect.ValueOf(nil)
		// setter.
		setterName := field.Tag.Get("setter")
		if len(setterName) >= 1 {
			setter = addressableVal.MethodByName(setterName)
		}
		// help
		help = tag.Get("help")
		// repr
		f := reflect.Indirect(aVal).FieldByName(field.Name)
		repr = toReprFromReflectValue(f)
		//
		m.Set(configPath, &ConfigFieldMirror{
			Help:   help,
			Repr:   repr,
			Setter: setter,
		})
		//
		return nil
	})
	//
	return &ConfigMirror{M: m, C: c}
}

func toReprFromReflectValue(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Bool:
		b := v.Bool()
		return fmt.Sprintf("%v", b)
	case reflect.Int:
		n := v.Int()
		return fmt.Sprintf("%v", n)
	case reflect.String:
		return v.String()
	default:
		return v.String()
	}
}

func joinConfigPath(path []reflect.StructField) string {
	yamlNames := []string{}
	for _, field := range path {
		yamlNames = append(yamlNames, field.Tag.Get("yaml"))
	}
	return strings.Join(yamlNames, ".")
}

func makeConfigPathString(ancestors []reflect.StructField, field reflect.StructField) string {
	return joinConfigPath(append(ancestors, field))
}

func GetConfigMirror() *ConfigMirror {
	return configMirror
}

func (c *ConfigMirror) GetKeys() []string {
	iter := c.M.IterFunc()
	result := []string{}
	for kv, ok := iter(); ok; kv, ok = iter() {
		result = append(result, kv.Key.(string))
	}
	return result
}

func (c *ConfigMirror) GetHelp(key string) (string, error) {
	v, ok := c.M.Get(key)
	if ok {
		return v.(*ConfigFieldMirror).Help, nil
	}
	return "", ErrNotFound
}

func (c *ConfigMirror) GetRepr(key string) (string, error) {
	v, ok := c.M.Get(key)
	if ok {
		return v.(*ConfigFieldMirror).Repr, nil
	}
	return "", ErrNotFound
}

func (c *ConfigMirror) SetConfig(key string, val string) error {
	v, ok := c.M.Get(key)
	if ok {
		setter := v.(*ConfigFieldMirror).Setter
		if !setter.IsValid() {
			fmt.Println("INVALID SETTER!!!")
			return ErrNotImplemented
		}
		_ = setter.Call([]reflect.Value{reflect.ValueOf(val)})
		return nil
	}
	return ErrNotFound
}
