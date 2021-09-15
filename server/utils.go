package main

import (
	"encoding/json"
	"fmt"
	"time"
	// "github.com/mattermost/mattermost-server/v5/model"
)

// Make a deep copy from src into dst.
func DeepCopyBorrow(dst *Borrow, src *Borrow) error {
	if dst == nil {
		return fmt.Errorf("dst cannot be nil")
	}
	if src == nil {
		return fmt.Errorf("src cannot be nil")
	}
	bytes, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("Unable to marshal src: %s", err)
	}
	err = json.Unmarshal(bytes, dst)
	if err != nil {
		return fmt.Errorf("Unable to unmarshal into dst: %s", err)
	}
	return nil
}

func DeepCopyBorrowRequest(dst *BorrowRequest, src *BorrowRequest) error {
	if dst == nil {
		return fmt.Errorf("dst cannot be nil")
	}
	if src == nil {
		return fmt.Errorf("src cannot be nil")
	}
	bytes, err := json.Marshal(*src)
	if err != nil {
		return fmt.Errorf("Unable to marshal src: %s", err)
	}
	err = json.Unmarshal(bytes, dst)
	if err != nil {
		return fmt.Errorf("Unable to unmarshal into dst: %s", err)
	}
	return nil
}

func ConvertStringArrayToSet(arr []string) map[string]bool {
	set := map[string]bool{}
	for _, v := range arr {
		set[v] = true
	}
	return set
}

func ConstainsInStringSet(set map[string]bool, values []string) bool {
	for _, v := range values {
		if _, ok := set[v]; ok {
			return true
		}
	}
	return false
}

func DeepCopy(dst interface{}, src interface{}) error {
	if dst == nil {
		return fmt.Errorf("dst cannot be nil")
	}
	if src == nil {
		return fmt.Errorf("src cannot be nil")
	}
	bytes, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("Unable to marshal src: %s", err)
	}
	err = json.Unmarshal(bytes, dst)
	if err != nil {
		return fmt.Errorf("Unable to unmarshal into dst: %s", err)
	}
	return nil
}

func GetNowTime() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
