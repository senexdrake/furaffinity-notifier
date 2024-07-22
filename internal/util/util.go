package util

import "strings"

const EnvPrefix = "FN_"

var truthyValues = []string{"1", "true", "yes", "on", "enable"}

func Map[T, U any](ts []T, f func(T) U) []U {
	us := make([]U, len(ts))
	for i := range ts {
		us[i] = f(ts[i])
	}
	return us
}

func Filter[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

func TrimHtmlText(s string) string {
	return strings.Trim(s, "\n ")
}

func PrefixEnvVar(s string) string {
	return EnvPrefix + s
}

func TruthyValues() []string {
	return truthyValues
}
