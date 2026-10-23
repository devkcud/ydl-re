package config

import "slices"

func (c Config) Set(scope Scope, key Key, value Value) (Config, bool, error) {
	return c.edit(scope, key, value, false)
}

func (c Config) Unset(scope Scope, key Key) (Config, bool, error) {
	return c.edit(scope, key, Value{}, true)
}

func (c Config) edit(scope Scope, key Key, value Value, remove bool) (Config, bool, error) {
	if err := c.Validate(); err != nil {
		return Config{}, false, err
	}
	if !slices.Contains(keys[:], key) {
		return Config{}, false, ErrKey
	}
	if !remove {
		if err := key.validate(value); err != nil {
			return Config{}, false, err
		}
	}

	i, err := c.scopeIndex(scope)
	if err != nil {
		return Config{}, false, err
	}

	next := c.Clone()
	values := &next.Defaults
	if i >= 0 {
		values = &next.Profiles[i].Settings
	}
	old, found := (*values)[key]
	changed := found
	if remove {
		delete(*values, key)
	} else {
		changed = !found || old != value
		if *values == nil {
			*values = make(Overrides)
		}
		(*values)[key] = value
	}

	if err := next.Validate(); err != nil {
		return Config{}, false, err
	}

	return next, changed, nil
}
