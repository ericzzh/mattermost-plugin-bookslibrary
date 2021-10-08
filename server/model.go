package main

import "github.com/pkg/errors"

type Book struct {
	*BookPublic
	*BookPrivate
	*BookInventory
	*Upload
}

type Books []Book

const (
	REL_BOOK_PUBLIC    = "public"
	REL_BOOK_PRIVATE   = "private"
	REL_BOOK_INVENTORY = "inventory"
)

const (
	TAG_PREFIX_BORROWER  = "#b_"
	TAG_PREFIX_LIBWORKER = "#w_"
	TAG_PREFIX_KEEPER    = "#k_"
	TAG_PREFIX_STATUS    = "#s_"
	TAG_PREFIX_ID        = "#id_"
	TAG_PREFIX_C1        = "#c1_"
	TAG_PREFIX_C2        = "#c2_"
	TAG_PREFIX_C3        = "#c3_"
)

type Relations map[string]string
type BookPublic struct {
	Id                string    `json:"id_pub"`
	Name              string    `json:"name_pub"`
	NameEn            string    `json:"name_en"`
	Category1         string    `json:"category1"`
	Category2         string    `json:"category2"`
	Category3         string    `json:"category3"`
	Author            string    `json:"author"`
	AuthorEn          string    `json:"author_en"`
	Translator        string    `json:"translator"`
	TranslatorEn      string    `json:"translator_en"`
	Publisher         string    `json:"publisher"`
	PublisherEn       string    `json:"publisher_en"`
	PublishDate       string    `json:"publish_date"`
	Intro             string    `json:"introduction"`
	BookIndex         string    `json:"book_index"`
	LibworkerUsers    []string  `json:"libworker_users"`
	LibworkerNames    []string  `json:"libworker_names,omitempty"`
	IsAllowedToBorrow bool      `json:"isAllowedToBorrow"`
	Tags              []string  `json:"tags,omitempty"`
	Relations         Relations `json:"relations_pub,omitempty"`
}

type BookPrivate struct {
	Id          string    `json:"id_pri,omitempty"`
	Name        string    `json:"name_pri,omitempty"`
	KeeperUsers []string  `json:"keeper_users"`
	KeeperNames []string  `json:"keeper_names,omitempty"`
	Relations   Relations `json:"relations_pri,omitempty"`
}

type BookInventory struct {
	Id          string    `json:"id_inv,omitempty"`
	Name        string    `json:"name_inv,omitempty"`
	Stock       int       `json:"stock"`
	TransmitOut int       `json:"transmit_out"`
	Lending     int       `json:"lending"`
	TransmitIn  int       `json:"transmit_in"`
	Relations   Relations `json:"relations_inv,omitempty"`
}

type Upload struct {
	Post_id string `json:"post_id"`
	Delete  bool   `json:"delete"`
}

const (
	BOOKS_ACTION_UPLOAD = "UPLOAD"
)

type BooksRequest struct {
	Action  string `json:"action"`
	ActUser string `json:"act_user"`
	Body    string `json:"body"`
}

const (
	MASTER    = "MASTER"
	BORROWER  = "BORROWER"
	LIBWORKER = "LIBWORKER"
	KEEPER    = "KEEPER"
)

const (
	STATUS_REQUESTED        = "R"
	STATUS_CONFIRMED        = "C"
	STATUS_DELIVIED         = "D"
	STATUS_RENEW_REQUESTED  = "RR"
	STATUS_RENEW_CONFIRMED  = "RC"
	STATUS_RETURN_REQUESTED = "RTR"
	STATUS_RETURN_CONFIRMED = "RTC"
	STATUS_RETURNED         = "RT"
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
	Delete        bool   `json:"delete"`
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
	RenewedTimes  int      `json:"renewed_times"`
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

type BookConfig struct {
	MaxRenewTimes int `json:"max_renew_times"`
	ExpiredDays   int `json:"expired_days"`
}

const (
	BOOK_UPLOAD_ERROR = "error"
	BOOK_UPLOAD_SUCC  = "ok"
)

type BooksMessage struct {
	PostId  string `json:"post_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type Messages map[string]string

type Result struct {
	Error    string   `json:"error"`
	Messages Messages `json:"messages,omitempty"`
}

var (
	ErrBorrowingLimited = errors.New("borrowing-book-limited")
	ErrLocked           = errors.New("record-locked")
	ErrNotFound         = errors.New("not-found")
	ErrNoStock          = errors.New("no-stock")
	ErrRenewLimited     = errors.New("renew-limited")
)
