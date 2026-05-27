package config

// Clone returns a deep copy of cfg.
func (cfg *Config) Clone() *Config {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	clone.Hosts = cloneHosts(cfg.Hosts)
	clone.Cases = cloneCases(cfg.Cases)
	clone.Stages = cloneStages(cfg.Stages)
	if cfg.Benchmark.Logging != nil {
		logging := *cfg.Benchmark.Logging
		clone.Benchmark.Logging = &logging
	}
	if cfg.Benchmark.Git != nil {
		git := *cfg.Benchmark.Git
		if cfg.Benchmark.Git.Capture != nil {
			capture := *cfg.Benchmark.Git.Capture
			git.Capture = &capture
		}
		clone.Benchmark.Git = &git
	}
	if cfg.Benchmark.Sync != nil {
		syncConfig := *cfg.Benchmark.Sync
		syncConfig.Args = append([]string(nil), cfg.Benchmark.Sync.Args...)
		clone.Benchmark.Sync = &syncConfig
	}
	return &clone
}

func cloneHosts(hosts map[string]Host) map[string]Host {
	if hosts == nil {
		return nil
	}
	clone := make(map[string]Host, len(hosts))
	for alias, host := range hosts {
		clone[alias] = host
	}
	return clone
}

func cloneCases(cases []Case) []Case {
	clone := make([]Case, len(cases))
	for i, benchmarkCase := range cases {
		clone[i] = benchmarkCase
		clone[i].Env = cloneStringMap(benchmarkCase.Env)
	}
	return clone
}

func cloneStages(stages []Stage) []Stage {
	clone := make([]Stage, len(stages))
	for i, stage := range stages {
		clone[i] = stage
		clone[i].Hosts = append([]string(nil), stage.Hosts...)
		clone[i].Outputs = cloneOutputs(stage.Outputs)
		if stage.HealthCheck != nil {
			healthCheck := *stage.HealthCheck
			clone[i].HealthCheck = &healthCheck
		}
	}
	return clone
}

func cloneOutputs(outputs []Output) []Output {
	return append([]Output(nil), outputs...)
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}
