package l2contractdeployer

// Option is a function that applies configs to a Config Object
type Option = func(c *Config)

// Config holds the properties that configure the package
type Config struct {
	l1HTTPURL         string
	l1privateKey      string
	l2Port            int
	l2Host            string
	l2PrivateKey      string
	hocPKString       string
	pocPKString       string
	messageBusAddress string
	dockerImage       string
}

func NewContractDeployerConfig(opts ...Option) *Config {
	defaultConfig := &Config{}

	for _, opt := range opts {
		opt(defaultConfig)
	}

	return defaultConfig
}

func WithL1HTTPURL(s string) Option {
	return func(c *Config) {
		c.l1HTTPURL = s
	}
}

func WithL1PrivateKey(s string) Option {
	return func(c *Config) {
		c.l1privateKey = s
	}
}

func WithL2WSPort(i int) Option {
	return func(c *Config) {
		c.l2Port = i
	}
}

func WithL2Host(s string) Option {
	return func(c *Config) {
		c.l2Host = s
	}
}

func WithMessageBusContractAddress(s string) Option {
	return func(c *Config) {
		c.messageBusAddress = s
	}
}

func WithL2PrivateKey(s string) Option {
	return func(c *Config) {
		c.l2PrivateKey = s
	}
}

func WithDockerImage(s string) Option {
	return func(c *Config) {
		c.dockerImage = s
	}
}

func WithHocPKString(s string) Option {
	return func(c *Config) {
		c.hocPKString = s
	}
}

func WithPocPKString(s string) Option {
	return func(c *Config) {
		c.pocPKString = s
	}
}
