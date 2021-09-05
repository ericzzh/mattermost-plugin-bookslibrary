package main

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

const (
	MASTER    = "MASTER"
	BORROWER  = "BORROWER"
	LIBWORKER = "LIBWORKER"
	KEEPER    = "KEEPER"
)

const (
	STATUS_REQUESTED = "REQUESTED"
	STATUS_CONFIRMED = "CONFIRMED"
	STATUS_DELIVIED  = "DELIVIED"
	STATUS_RENEWED   = "RENEWED"
	STATUS_RETURNED  = "RETURNED"
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

//The key role is library worker(libworker). it is the cross-point in the workflow
//A library worker is a connected point bewteen a borrower and a keeper.
//He/She should cooridinate the workflow.
//A borrower and a book keeper has no direct connection.
//Because the work may become heavy for a library work,
//every book can assgin multi-workers, but there should be **ONLY ONE** worker be assigned
//in a borrowing workflow. We use a simple random number(uniform distribution) solution to solve this case.
//To be more flexible a book are degsined to be able to assgin multi-persons too.
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
	RequestDate   int64    `json:"request_date"`
	ConfirmDate   int64    `json:"confirm_date"`
	DeliveryDate  int64    `json:"delivery_date"`
	RenewDate     int64    `json:"renew_date"`
	ReturnDate    int64    `json:"return_date"`
	WorkflowType  string   `json:"workflow_type"`
	Worflow       []string `json:"workflow"`
	Status        string   `json:"status"`
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
