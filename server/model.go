package main

import "github.com/pkg/errors"

type Book struct {
	Id                string   `json:"id"`
	Name              string   `json:"name"`
	NameEn            string   `json:"name_en"`
	Category1         string   `json:"category1"`
	Category2         string   `json:"category2"`
	Category3         string   `json:"category3"`
	Author            string   `json:"author"`
	AuthorEn          string   `json:"author_en"`
	Translator        string   `json:"translator"`
	TranslatorEn      string   `json:"translator_en"`
	Publisher         string   `json:"publisher"`
	PublisherEn       string   `json:"publisher_en"`
	PublishDate       string   `json:"publish_date"`
	Intro             string   `json:"introduction"`
	BookIndex         string   `json:"book_index"`
	LibworkerUsers    []string `json:"libworker_users"`
	LibworkerNames    []string `json:"libworker_names"`
	KeeperUsers       []string `json:"keeper_users"`
	KeeperNames       []string `json:"keeper_names"`
	IsAllowedToBorrow bool     `json:"isAllowedToBorrow"`
	Tags              []string `json:"tags"`
}

type BookPublic struct {
	Id                string            `json:"id"`
	Name              string            `json:"name"`
	NameEn            string            `json:"name_en"`
	Category1         string            `json:"category1"`
	Category2         string            `json:"category2"`
	Category3         string            `json:"category3"`
	Author            string            `json:"author"`
	AuthorEn          string            `json:"author_en"`
	Translator        string            `json:"translator"`
	TranslatorEn      string            `json:"translator_en"`
	Publisher         string            `json:"publisher"`
	PublisherEn       string            `json:"publisher_en"`
	PublishDate       string            `json:"publish_date"`
	Intro             string            `json:"introduction"`
	BookIndex         string            `json:"book_index"`
	LibworkerUsers    []string          `json:"libworker_users"`
	LibworkerNames    []string          `json:"libworker_names"`
	IsAllowedToBorrow bool              `json:"isAllowedToBorrow"`
	Tags              []string          `json:"tags"`
	Relations         map[string]string `json:"relations"`
}

type BookPrivate struct{
	Id                string            `json:"id"`
	Name              string            `json:"name"`
	Relations         map[string]string `json:"relations"`
}

const (
	MASTER    = "MASTER"
	BORROWER  = "BORROWER"
	LIBWORKER = "LIBWORKER"
	KEEPER    = "KEEPER"
)

const (
	STATUS_REQUESTED        = "REQUESTED"
	STATUS_CONFIRMED        = "CONFIRMED"
	STATUS_DELIVIED         = "DELIVIED"
	STATUS_RENEW_REQUESTED  = "RENEW_REQUESTED"
	STATUS_RENEW_CONFIRMED  = "RENEW_CONFIRMED"
	STATUS_RETURN_REQUESTED = "RETURN_REQUESTED"
	STATUS_RETURN_CONFIRMED = "RETURN_CONFIRMED"
	STATUS_RETURNED         = "RETURNED"
)

const (
	WORKFLOW_BORROW = "BORROW"
	WORKFLOW_RENEW  = "RENEW"
	WORKFLOW_RETURN = "RETURN"
)

type BorrowRequestKey struct {
	BookPostId   string `json:"book_post_id"`
	BorrowerUser string `json:"borrower_user"`
}

type WorkflowRequest struct {
	MasterPostKey string `json:"master_key"`
	ActorUser     string `json:"act_user"`
	NextStepIndex int    `json:"next_step_index"`
}

//The key role is library worker(libworker). it is the cross-point in the workflow
//A library worker is a connected point bewteen a borrower and a keeper.
//He/She should cooridinate the workflow.
//A borrower and a book keeper has no direct connection.
//Because the work may become heavy for a library work,
//every book can assgin multi-workers, but there should be **ONLY ONE** worker be assigned
//in a borrowing workflow. We use a simple random number(uniform distribution) solution to solve this case.
//To be more flexible a book are degsined to be able to assgin multi-persons too.
type Step struct {
	WorkflowType  string   `json:"workflow_type"`
	Status        string   `json:"status"`
	ActorRole     string   `json:"actor_role"`
	Completed     bool     `json:"completed"`
	ActionDate    int64    `json:"action_date"`
	NextStepIndex []int    `json:"next_step_index"`
	RelatedRoles  []string `json:"related_roles"`
}

type BorrowRequest struct {
	BookPostId    string   `json:"book_post_id"`
	BookId        string   `json:"book_id"`
	BookName      string   `json:"book_name"`
	Author        string   `json:"author"`
	BorrowerUser  string   `json:"borrower_user"`
	BorrowerName  string   `json:"borrower_name"`
	LibworkerUser string   `json:"libworker_user"`
	LibworkerName string   `json:"libworker_name"`
	KeeperUsers   []string `json:"keeper_users,omitempty"`
	KeeperNames   []string `json:"keeper_names,omitempty"`
	Worflow       []Step   `json:"workflow"`
	StepIndex     int      `json:"step_index"`
	LastStepIndex int      `json:"last_step_index"`
	Tags          []string `json:"tags"`
}

type Borrow struct {
	DataOrImage  *BorrowRequest `json:"dataOrImage"`
	Role         []string       `json:"role"`
	RelationKeys RelationKeys   `json:"relations_keys"`
}

type RelationKeys struct {
	Book      string   `json:"book"`
	Master    string   `json:"master,omitempty"`
	Borrower  string   `json:"borrower,omitempty"`
	Libworker string   `json:"libworker,omitempty"`
	Keepers   []string `json:"keepers,omitempty"`
}

type Result struct {
	Error string `json:"error"`
}

var (
	ErrBorrowingLimited = errors.New("borrowing-book-limited")
)
