package envflag

import (
	"flag"
	"log"
	"os"
	"regexp"
	"strings"
)

type envValue struct {
	flag.Value
	envName  string
	envValue string
	set      bool
}

func (v *envValue) Set(value string) error {
	v.set = true
	return v.Value.Set(value)
}

func Parse() error {
	return ParseFlagSet(flag.CommandLine, os.Args[1:])
}

type boolFlag interface {
	IsBoolFlag() bool
}

func ParseFlagSet(flagSet *flag.FlagSet, arguments []string) error {
	flagSet.VisitAll(func(f *flag.Flag) {
		envName := ToSnakeCase(f.Name)
		if env := os.Getenv(envName); env != "" {
			if b, ok := f.Value.(boolFlag); ok && b.IsBoolFlag() {
				f.Value = &envValue{Value: f.Value, envName: envName, envValue: env}
			}
		}
	})

	if err := flagSet.Parse(arguments); err != nil {
		return err
	}

	flagSet.VisitAll(func(f *flag.Flag) {
		if ev, ok := f.Value.(*envValue); ok && !ev.set {
			if err := ev.Value.Set(ev.envValue); err != nil {
				log.Printf("set %s = %s ($%s) failed: %v", f.Name, ev.envName, ev.envValue, err)
			}
		}
	})

	return nil
}

// ToSnakeCase converts camelCase to snaked CAMEL_CASE.
func ToSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToUpper(snake)
}

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)
