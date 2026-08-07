package schema

import "fmt"

// oneOfStrings 返回只允许指定字符串值的 Ent 字段校验器。
func oneOfStrings(values ...string) func(string) error {
	allowed := make(map[string]struct{}, len(values))
	for _, value := range values {
		allowed[value] = struct{}{}
	}
	return func(value string) error {
		if _, ok := allowed[value]; ok {
			return nil
		}
		return fmt.Errorf("value %q is not allowed", value)
	}
}
