package main

import (
	"fmt"
	"reflect"
	"testing"
)

var _ = fmt.Println

func TestDeepCopyBorrow(t *testing.T) {

	bq := &BorrowRequest{
		BookPostId:  "bookid-1",
		KeeperUsers: []string{"kpuser1", "kpuser2"},
	}

	bqDst := &BorrowRequest{}
	err := DeepCopyBorrowRequest(bqDst, bq)

	if err != nil {
		t.Errorf("Error shouldnot occur, %v", err)
	}

	if bq == bqDst {
		t.Errorf("shouldnot be the same object , %v, %v", bq, bqDst)
	}

        if !reflect.DeepEqual(bq, bqDst) {

		t.Errorf("should have alues , %v, %v", *bq, *bqDst)
        }


}

