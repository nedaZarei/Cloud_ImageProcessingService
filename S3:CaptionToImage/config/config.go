package config

import "github.com/spf13/viper"

type Config struct {
	Server      Server      `yaml:"server"`
	Postgres    Postgres    `yaml:"postgres"`
	Minio       Minio       `yaml:"minio"`
	HuggingFace HuggingFace `yaml:"huggingface"`
	Email       Email       `yaml:"email"`
}

type Server struct {
	Port string `yaml:"port"`
}

type Postgres struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	Database        string `yaml:"database"`
	AutoCreateTable bool   `yaml:"auto_create"`
}

type Minio struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Bucket    string `yaml:"bucket"`
}

type HuggingFace struct {
	APIKey string
	URL    string
}

type Email struct {
	APIKey string
	From   string
}

func InitConfig(filename string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(filename)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
