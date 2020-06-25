package islb

import (
	"encoding/json"
	fmt "fmt"
	"strings"
)

// MarshalNodeField for storage in redis
func (n NodeInfo) MarshalNodeField() (string, string, error) {
	value, err := json.Marshal(n)
	if err != nil {
		return "node/" + n.Id, "", fmt.Errorf("Marshal: %v", err)
	}
	return "node/" + n.Id, string(value), nil
}

// UnmarshalNodeField from string
func UnmarshalNodeField(key string, value string) (*NodeInfo, error) {
	var node NodeInfo
	if err := json.Unmarshal([]byte(value), &node); err != nil {
		return nil, fmt.Errorf("Unmarshal: %v", err)
	}
	return &node, nil
}

// BuildKey from UserInfo
func (u UserInfo) BuildKey() string {
	strs := []string{u.Dc, string(u.Rid), "user", "info", string(u.Uid)}
	return strings.Join(strs, "/")
}

// ParseUserInfoKey into UserInfo stfuct
func ParseUserInfoKey(key string) (*UserInfo, error) {
	var info UserInfo
	arr := strings.Split(key, "/")
	if len(arr) != 5 {
		return nil, fmt.Errorf("Can‘t parse userinfo; [%s]", key)
	}
	info.Dc = arr[0]
	info.Rid = arr[1]
	info.Uid = arr[4]
	return &info, nil
}
