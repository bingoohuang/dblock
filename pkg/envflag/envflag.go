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
	boolFlag bool
}

func (v *envValue) IsBoolFlag() bool {
	return v.boolFlag
}

func (v *envValue) Set(value string) error {
	v.set = true
	return v.Value.Set(value)
}

func Parse() error {
	return ParseFlagSet(flag.CommandLine, os.Args[1:])
}

// optional interface to indicate boolean flags that can be
// supplied without "=value" text
type boolFlag interface {
	flag.Value
	IsBoolFlag() bool
}

func ParseFlagSet(flagSet *flag.FlagSet, arguments []string) error {
	flagSet.VisitAll(func(f *flag.Flag) {
		envName := ToSnakeCase(f.Name)
		if env := os.Getenv(envName); env != "" {
			b, ok := f.Value.(boolFlag)
			f.Value = &envValue{
				Value:    f.Value,
				envName:  envName,
				envValue: env,
				boolFlag: ok && b.IsBoolFlag(),
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
