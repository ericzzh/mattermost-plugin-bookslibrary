package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
)

type updateOptions struct {
	pub     *BookPublic
	pubPost *model.Post
	pri     *BookPrivate
	priPost *model.Post
	inv     *BookInventory
	invPost *model.Post
}

type bookInfo struct {
	book    *Book
	pubPost *model.Post
	priPost *model.Post
	invPost *model.Post
}

func (p *Plugin) GetABook(id string) (*bookInfo, error) {

	bookPost, appErr := p.API.GetPost(id)

	if appErr != nil {
		return nil, errors.Wrapf(appErr, "Failed to get book post id(pub) %s.", id)
	}

	pubPost := bookPost

	var bookPub BookPublic
	if err := json.Unmarshal([]byte(bookPost.Message), &bookPub); err != nil {
		return nil, errors.Wrapf(err, "Failed to unmarshal bookpost. post id(pub):%s", id)
	}

	priId := bookPub.Relations[REL_BOOK_PRIVATE]

	bookPost, appErr = p.API.GetPost(priId)

	if appErr != nil {
		return nil, errors.Wrapf(appErr, "Failed to get book post id(pri) %s.", id)
	}

	priPost := bookPost

	var bookPri BookPrivate
	if err := json.Unmarshal([]byte(bookPost.Message), &bookPri); err != nil {
		return nil, errors.Wrapf(err, "Failed to unmarshal bookpost. post id(pri):%s", id)
	}

	invId := bookPub.Relations[REL_BOOK_INVENTORY]

	bookPost, appErr = p.API.GetPost(invId)

	if appErr != nil {
		return nil, errors.Wrapf(appErr, "Failed to get book post id(inv) %s.", id)
	}

	invPost := bookPost

	var bookInv BookInventory
	if err := json.Unmarshal([]byte(bookPost.Message), &bookInv); err != nil {
		return nil, errors.Wrapf(err, "Failed to unmarshal bookpost. post id(inv):%s", id)
	}
	return &bookInfo{
		&Book{
			&bookPub,
			&bookPri,
			&bookInv,
			nil,
		},
		pubPost,
		priPost,
		invPost,
	}, nil
}

func (p *Plugin) handleBooksRequest(c *plugin.Context, w http.ResponseWriter, r *http.Request) {

	var booksRequest *BooksRequest
	err := json.NewDecoder(r.Body).Decode(&booksRequest)
	if err != nil {
		p.API.LogError("Failed to convert from book request.", "err", fmt.Sprintf("%+v", err))
		resp, _ := json.Marshal(Result{
			Error: "Failed to convert from book request.",
		})

		w.Write(resp)
		return

	}

	switch booksRequest.Action {
	case BOOKS_ACTION_UPLOAD:

		var messages Messages

		messages, err := p._uploadBooks(booksRequest.Body)
		if err != nil {
			p.API.LogError("upload books error.", "err", fmt.Sprintf("%+v", err))
			resp, _ := json.Marshal(Result{
				Error:    "upload books error.",
				Messages: messages,
			})

			w.Write(resp)
			return
		}

		resp, _ := json.Marshal(Result{
			Error:    "",
			Messages: messages,
		})

		w.Write(resp)

	default:
		p.API.LogError("invalidate action.")
		resp, _ := json.Marshal(Result{
			Error: "invalidate action.",
		})

		w.Write(resp)
		return
	}
}

func (p *Plugin) _uploadBooks(booksJson string) (Messages, error) {

	var (
		books  []*Book
		retErr error
	)

	messages := Messages{}

	if err := json.Unmarshal([]byte(booksJson), &books); err != nil {
		return nil, errors.Wrapf(err, "convert to books error.")
	}

	for _, book := range books {
		bookmsg, err := p._uploadABook(book)
		if err != nil {
			retErr = errors.New("some error was occurred in books.")
		}
		mj, _ := json.Marshal(bookmsg)
		messages[book.BookPublic.Id] = string(mj)
	}

	return messages, retErr
}

func (p *Plugin) _uploadABook(book *Book) (*BooksMessage, error) {

	var bookupl *Upload
	if book.Upload == nil {
		bookupl = &Upload{}
	} else {
		bookupl = book.Upload
	}

	if bookupl.Post_id != "" {
		if bookupl.Delete == true {
			//---------------------------------------
			// Delete a exsited post
			//---------------------------------------
			if err := p._deleteABook(book); err != nil {
				return &BooksMessage{
					PostId:  book.Upload.Post_id,
					Status:  BOOK_UPLOAD_ERROR,
					Message: err.Error(),
				}, err
			}
			return &BooksMessage{
				PostId:  bookupl.Post_id,
				Status:  BOOK_UPLOAD_SUCC,
				Message: "Successfully deleted.",
			}, nil
		} else {
			//---------------------------------------
			// Update a exsited post
			//---------------------------------------
			err := p._updateABook(book)
			if err != nil {
				return &BooksMessage{
					PostId:  bookupl.Post_id,
					Status:  BOOK_UPLOAD_ERROR,
					Message: err.Error(),
				}, err
			}
			return &BooksMessage{
				PostId:  bookupl.Post_id,
				Status:  BOOK_UPLOAD_SUCC,
				Message: "Successfully updated.",
			}, nil
		}
	}
	//---------------------------------------
	// Create a  post
	//---------------------------------------
	pid, err := p._createABook(book)
	if err != nil {
		return &BooksMessage{
			PostId:  "",
			Status:  BOOK_UPLOAD_ERROR,
			Message: err.Error(),
		}, err
	}
	return &BooksMessage{
		PostId:  pid,
		Status:  BOOK_UPLOAD_SUCC,
		Message: "Successfully created.",
	}, nil

}

func (p *Plugin) _updateBookParts(opts updateOptions) error {

	updates := []*model.Post{}

	if opts.pub != nil {
		if err := p._updateBookPart(opts.pubPost, opts.pub); err != nil {
			p._rollbackToOld(updates)
			return err
		}
		updates = append(updates, opts.pubPost)
	}

	if opts.pri != nil {
		if err := p._updateBookPart(opts.priPost, opts.pri); err != nil {
			p._rollbackToOld(updates)
			return err
		}
		updates = append(updates, opts.priPost)
	}

	if opts.inv != nil {
		if err := p._updateBookPart(opts.invPost, opts.inv); err != nil {
			p._rollbackToOld(updates)
			return err
		}
		updates = append(updates, opts.invPost)
	}
	return nil
}

func (p *Plugin) _updateBookPart(post *model.Post, part interface{}) error {
	mjson, err := json.MarshalIndent(part, "", "  ")
	if err != nil {
		return err
	}

	newPost := &model.Post{}
	DeepCopy(newPost, post)
	if newPost.Message != string(mjson) {
		newPost.Message = string(mjson)
		if _, appErr := p.API.UpdatePost(newPost); appErr != nil {
			return appErr
		}

	}

	return nil
}

func (p *Plugin) _updateABook(book *Book) error {

	pubId := book.Upload.Post_id

	//lock pub part only
	if _, ok := lockmap.LoadOrStore(pubId, struct{}{}); ok {
		return errors.New(fmt.Sprintf("lock error."))
	}

	defer lockmap.Delete(pubId)

	if err := p._fillABookCommon(book); err != nil {
		return errors.Wrapf(err, "fill error.")
	}

	//------------------------------
	//get public part
	//------------------------------
	bookPub := book.BookPublic

	var (
		bookPubOldPost *model.Post
	)
	bookPubOld := &BookPublic{}
	bookPubOldPost, err := p._getUnmarshaledPost(pubId, bookPubOld)
	if err != nil {
		return errors.Wrapf(err, "get pub error.")
	}
	//IsAllowedToBorrow is not updated when updating
	if !book.Upload.UpdIsAllowedToBorrow {
		bookPub.IsAllowedToBorrow = bookPubOld.IsAllowedToBorrow
                bookPub.ManuallyDisallowed = bookPubOld.ManuallyDisallowed
	} else {
		//manually update, should update this field
		if bookPub.IsAllowedToBorrow {
			bookPub.ManuallyDisallowed = false
                        bookPub.ReasonOfDisallowed = ""
		} else {
			bookPub.ManuallyDisallowed = true
		}
	}
	bookPub.Relations = bookPubOld.Relations
	//------------------------------
	//get private part
	//------------------------------
	bookPri := book.BookPrivate

	var (
		bookPriOldPost *model.Post
	)

	bookPriOld := &BookPrivate{}
	priId := bookPubOld.Relations[REL_BOOK_PRIVATE]
	bookPriOldPost, err = p._getUnmarshaledPost(priId, bookPriOld)
	if err != nil {
		return errors.Wrapf(err, "get pri error.")
	}
	if bookPri != nil {
		bookPri.Relations = bookPriOld.Relations
	}

	//------------------------------
	//get inventory part
	//------------------------------
	bookInv := book.BookInventory

	var (
		bookInvOldPost *model.Post
	)

	bookInvOld := &BookInventory{}
	invId := bookPubOld.Relations[REL_BOOK_INVENTORY]
	bookInvOldPost, err = p._getUnmarshaledPost(invId, bookInvOld)
	if err != nil {
		return errors.Wrapf(err, "get inv error.")
	}

	if bookInv != nil {
		totalOld := bookInvOld.Stock + bookInvOld.TransmitOut + bookInvOld.Lending + bookInvOld.TransmitIn
		if bookInv.Stock > totalOld {
			diff := bookInv.Stock - totalOld
			bookInv.Stock = bookInvOld.Stock + diff
		} else {
			diff := totalOld - bookInv.Stock
			bookInv.Stock = bookInvOld.Stock - diff
			if bookInv.Stock < 0 {
				return errors.New("stock can not be negative.")
			}
		}
		bookInv.TransmitIn = bookInvOld.TransmitIn
		bookInv.Lending = bookInvOld.Lending
		bookInv.TransmitOut = bookInvOld.TransmitOut
                
                if bookInv.Stock > 0 && !bookPub.ManuallyDisallowed && !bookPub.IsAllowedToBorrow {
                    bookPub.IsAllowedToBorrow = true
                }

		bookInv.Relations = bookInvOld.Relations
	}

	if err := p._updateBookParts(
		updateOptions{
			pub:     bookPub,
			pubPost: bookPubOldPost,
			pri:     bookPri,
			priPost: bookPriOldPost,
			inv:     bookInv,
			invPost: bookInvOldPost,
		},
	); err != nil {
		return errors.Wrapf(err, "update posts error.")
	}

	return nil
}

func (p *Plugin) _getUnmarshaledPost(id string, value interface{}) (*model.Post, error) {

	var appErr *model.AppError

	post, appErr := p.API.GetPost(id)
	if appErr != nil {
		if appErr.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, errors.Wrapf(appErr, fmt.Sprintf("get post error."))
	}

	err := json.Unmarshal([]byte(post.Message), value)
	if err != nil {
		return nil, errors.Wrapf(err, fmt.Sprintf("marshal error."))
	}

	return post, nil
}

type compareAndUpdateOptions struct {
	compare func() bool
}

// func (p *Plugin) _compareAndUpdate(oldPost *model.Post, newPart interface{}, options ...compareAndUpdateOptions) error {
//
// 	j, err := json.Marshal(newPart)
// 	if err != nil {
// 		return errors.Wrapf(err, "marshal error.")
// 	}
//
// 	if oldPost.Message != string(j) {
// 		newPost := *oldPost
// 		newPost.Message = string(j)
// 		_, appErr := p.API.UpdatePost(&newPost)
// 		if appErr != nil {
// 			return errors.Wrapf(appErr, "update error.")
// 		}
//
// 	}
// 	return nil
// }
func (p *Plugin) _deleteABook(book *Book) error {

	pubId := book.Upload.Post_id

	if pubId == "" {
		return errors.New("post id is required.")
	}

	//lock pub part only
	if _, ok := lockmap.LoadOrStore(pubId, struct{}{}); ok {
		return errors.New(fmt.Sprintf("lock error."))
	}

	defer lockmap.Delete(pubId)

	//------------------------------
	//get public part
	//------------------------------
	bookPubOld := new(BookPublic)
	bookPubOldPost, err := p._getUnmarshaledPost(pubId, bookPubOld)
	if err != nil {
		//fail to get pub is fatal, so just return
		return errors.Wrapf(err, "get pub error.")
	}

	//------------------------------
	//get private part
	//------------------------------
	priId := bookPubOld.Relations[REL_BOOK_PRIVATE]
	bookPriOld := new(BookPrivate)
	bookPriOldPost, err := p._getUnmarshaledPost(priId, bookPriOld)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return errors.Wrapf(err, "get pri error.")
		}
	}

	//------------------------------
	//get inventory part
	//------------------------------
	invId := bookPubOld.Relations[REL_BOOK_INVENTORY]
	bookInvOld := new(BookInventory)
	bookInvOldPost, err := p._getUnmarshaledPost(invId, bookInvOld)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return errors.Wrapf(err, "get inv error.")
		}
	}

	totalOld := bookInvOld.Stock + bookInvOld.TransmitOut + bookInvOld.Lending + bookInvOld.TransmitIn

	if totalOld != bookInvOld.Stock {
		return errors.New("all books should be returned before deletion.")
	}

	//------------------------------
	//Start deleting
	//------------------------------
	if bookInvOldPost != nil {
		if err := p.API.DeletePost(invId); err != nil {
			return errors.Wrapf(err, "delete inventory record error. record is broken!, please retry.")
		}
	}

	if bookPriOldPost != nil {
		if err := p.API.DeletePost(priId); err != nil {
			return errors.Wrapf(err, "delete private record error. record is broken!, please retry.")
		}
	}

	if bookPubOldPost != nil {
		//Because the broken records maybe ocurred, make the public record is the lastest to be deleted.
		//so as to retry the deletion
		if err := p.API.DeletePost(pubId); err != nil {
			return errors.Wrapf(err, "delete pub record error. record is broken!, please retry.")
		}
	}
	return nil
}

func (p *Plugin) _fillABookCommon(book *Book) error {
	//fill in place

	//public part
	bookpub := book.BookPublic
	bookpub.IsAllowedToBorrow = book.IsAllowedToBorrow
	bookpub.Tags = []string{
		TAG_PREFIX_ID + bookpub.Id,
		TAG_PREFIX_C1 + bookpub.Category1,
		TAG_PREFIX_C2 + bookpub.Category2,
		TAG_PREFIX_C3 + bookpub.Category3,
	}

	bookpub.LibworkerNames = []string{}
	for _, username := range bookpub.LibworkerUsers {
		disName, err := p._getDisplayNameByUser(username)
		if err != nil {
			return err
		}
		bookpub.LibworkerNames = append(bookpub.LibworkerNames, disName)
	}

	//private part
	bookpri := book.BookPrivate
	if bookpri != nil {
		bookpri.Id = bookpub.Id
		bookpri.Name = bookpub.Name

		bookpri.KeeperNames = []string{}
		for _, username := range bookpri.KeeperUsers {
			disName, err := p._getDisplayNameByUser(username)
			if err != nil {
				return err
			}
			bookpri.KeeperNames = append(bookpri.KeeperNames, disName)
		}
	}

	//inventory part
	bookinv := book.BookInventory
	if bookinv != nil {
		bookinv.Id = bookpub.Id
		bookinv.Name = bookpub.Name
	}

	return nil
}

func (p *Plugin) _createABook(book *Book) (string, error) {
	if err := p._fillABookCommon(book); err != nil {
		return "", errors.Wrapf(err, "fill a book error.")
	}

	if book.BookPublic == nil ||
		book.BookPrivate == nil ||
		book.BookInventory == nil {
		return "", errors.New("pub, pri or inv part should not be nil.")
	}

	//---------------------------------------
	// Create a  post
	//---------------------------------------
	// Create a empty post

	//start a simple transaction
	created := []*model.Post{}

	//Public
	postPub, appErr := p.API.CreatePost(
		&model.Post{
			UserId:    p.botID,
			Type:      "custom_book_type",
			ChannelId: p.booksChannel.Id,
			Message:   "",
		},
	)

	if appErr != nil {
		return "", errors.Wrapf(appErr, "create pub post error.")
	}

	created = append(created, postPub)

	//Private
	postPri, appErr := p.API.CreatePost(
		&model.Post{
			UserId:    p.botID,
			Type:      "custom_book_private_type",
			ChannelId: p.booksPriChannel.Id,
			Message:   "",
		},
	)

	if appErr != nil {
		p._rollBackCreated(created)
		return "", errors.Wrapf(appErr, "create pri post error.")
	}

	created = append(created, postPri)

	//inventory
	postInv, appErr := p.API.CreatePost(
		&model.Post{
			UserId:    p.botID,
			Type:      "custom_book_inventory_type",
			ChannelId: p.booksInvChannel.Id,
			Message:   "",
		},
	)

	if appErr != nil {
		p._rollBackCreated(created)
		return "", errors.Wrapf(appErr, "create inv post error.")
	}

	created = append(created, postInv)

	//---------------------------------------
	// Update post to be full
	//---------------------------------------

	//Public
	book.BookPublic.Relations = Relations{}
	book.BookPublic.Relations[REL_BOOK_PRIVATE] = postPri.Id
	book.BookPublic.Relations[REL_BOOK_INVENTORY] = postInv.Id

	//Private
	book.BookPrivate.Relations = Relations{}
	book.BookPrivate.Relations[REL_BOOK_PUBLIC] = postPub.Id

	//inventory
	book.BookInventory.Relations = Relations{}
	book.BookInventory.Relations[REL_BOOK_PUBLIC] = postPub.Id
	if err := p._updateBookParts(
		updateOptions{
			pub:     book.BookPublic,
			pubPost: postPub,
			pri:     book.BookPrivate,
			priPost: postPri,
			inv:     book.BookInventory,
			invPost: postInv,
		},
	); err != nil {
		p._rollBackCreated(created)
		return "", errors.Wrapf(err, "update created post error.")
	}

	return postPub.Id, nil
}
