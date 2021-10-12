type STATUS_REQUESTED = "R";
type STATUS_CONFIRMED = "C";
type STATUS_DELIVIED = "D";
type STATUS_RENEW_REQUESTED = "RR";
type STATUS_RENEW_CONFIRMED = "RC";
type STATUS_RETURN_REQUESTED = "RTR";
type STATUS_RETURN_CONFIRMED = "RTC";
type STATUS_RETURNED = "RT";
type STATUS =
    | STATUS_REQUESTED
    | STATUS_CONFIRMED
    | STATUS_DELIVIED
    | STATUS_RENEW_REQUESTED
    | STATUS_RENEW_CONFIRMED
    | STATUS_RETURN_REQUESTED
    | STATUS_RETURN_CONFIRMED
    | STATUS_RETURNED;

interface Book {
    id_pub: string;
    name_pub: string;
    name_en: string;
    category1: string;
    category2: string;
    category3: string;
    author: string;
    author_en: string;
    translator: string;
    translator_en: string;
    publisher: string;
    publisher_en: string;
    publish_date: string;
    introduction: string;
    book_index: string;
    libworker_users: string[];
    libworker_names: string[];
    isAllowedToBorrow: boolean;
    tags: string[];
    post_id: string;
    relations_pub: {
        private: string;
        inventory: string;
    };
}

interface BooksRequest {
    action: string;
    act_user: string;
    body: string;
}

interface BookMessage {
    post_id: string;
    status: string;
    message: string;
}

interface Messages {
    [key: string]: string;
}

interface Result {
    error: string;
    messages: Messages;
}

interface Step {
    workflow_type: string;
    status: STATUS;
    actor_role: string;
    completed: boolean;
    action_date: number;
    next_step_index: number[];
    related_roles: string[];
    last_step_index: number;
}

interface BorrowRequestKey {
    book_post_id: string;
    borrower_user: string;
}

interface BorrowRequest {
    book_post_id: string;
    book_id: string;
    book_name: string;
    author: string;
    borrower_user: string;
    borrower_name: string;
    libworker_user: string;
    libworker_name: string;
    keeper_users: string[];
    keeper_names: string[];
    workflow: Step[];
    step_index: number;
    tags: string[];
}

interface Borrow {
    // put the dataOrImage to be first so as to hide the record of Thread view
    dataOrImage: BorrowRequest;
    role: "MASTER" | "BORROWER" | "LIBWORKER" | "KEEPER";
    relations_keys: {
        book: string;
        master: string;
        borrower: string;
        libworker: string;
        keepers: string;
    };
}

interface WorkflowRequest {
    master_key: string;
    act_user: string;
    next_step_index?: number;
    delete?: boolean;
    backward?:boolean;
}
