package handlers

import (
	"encoding/json"
	"fmt"

	. "github.com/liverecord/server/common/frame"
	"github.com/liverecord/server/model"
)

func (Ctx *AppContext) CommentList(frame Frame) {
	var comments []model.Comment
	comments = make([]model.Comment, 0, 1)
	var topic model.Topic
	err := frame.BindJSON(&topic)
	if err != nil {
		Ctx.Logger.WithError(err)
		return
	}
	rows, err := Ctx.Db.
		Table("comments").
		Joins("JOIN users ON users.id = comments.user_id ").
		Joins("JOIN topics ON topics.id = comments.topic_id ").
		Joins("LEFT JOIN categories ON topics.category_id = categories.id ").
		Where("topic_id = ?", topic.ID).
		Group("comments.id").
		Order("comments.created_at DESC").
		Select("comments.*, " +
			"users.name as user_name, " +
			"users.slug as user_slug, " +
			"users.picture as user_picture, " +
			"users.rank as user_rank, " +
			"users.online as user_online, " +
			"topics.title as topic_title, " +
			"topics.slug as topic_slug, " +
			"topics.id as topic_id, " +
			"topics.category_id as category_id, " +
			"categories.slug as category_slug, " +
			"categories.name as category_name ").
		Rows()

	type CommentUser struct {
		UserSlug string
		UserName string
		UserRank float32
		UserOnline bool
		UserPicture string
	}

	type CommentTopic struct {
		TopicId    uint
		TopicSlug  string
		TopicTitle string
	}

	type CommentCategory struct {
		CategoryId   uint
		CategorySlug string
		CategoryName string
	}

	// comments

	if err == nil {
		for rows.Next() {
			var comm model.Comment
			var commTopic CommentTopic
			var commCat CommentCategory
			var commUser CommentUser

			if err := Ctx.Db.ScanRows(rows, &comm); err != nil {
				Ctx.Logger.Errorf("should get no error, but got %v", err)
			}

			if err := Ctx.Db.ScanRows(rows, &commUser); err == nil {
				comm.User.ID = comm.UserID
				comm.User.Slug = commUser.UserSlug
				comm.User.Name = commUser.UserName
				comm.User.Online = commUser.UserOnline
				comm.User.Picture = commUser.UserPicture
				comm.User.Rank = commUser.UserRank
			}

			if err := Ctx.Db.ScanRows(rows, &commTopic); err == nil {
				comm.Topic.ID = comm.TopicID
				comm.Topic.Slug = commTopic.TopicSlug
				comm.Topic.Title = commTopic.TopicTitle
				Ctx.Logger.Debugln(commTopic)
			}

			if err := Ctx.Db.ScanRows(rows, &commCat); err == nil {
				comm.Topic.CategoryID = commCat.CategoryId
				comm.Topic.Category.ID = commCat.CategoryId
				comm.Topic.Category.Slug = commCat.CategorySlug
				comm.Topic.Category.Name = commCat.CategoryName
				Ctx.Logger.Debugln(commCat)
			}

			comments = append(comments, comm)
		}
		defer rows.Close()
		cats, _ := json.Marshal(comments)
		Ctx.Ws.WriteJSON(Frame{Type: CommentListFrame, Data: string(cats)})
	} else {
		Ctx.Logger.WithError(err)
	}
}

func (Ctx *AppContext) CommentSave(frame Frame) {
	if Ctx.IsAuthorized() {
		var comment model.Comment
		err := frame.BindJSON(&comment)
		Ctx.Logger.Info("Decoded comment", comment)
		Ctx.Logger.Info("User", Ctx.User)
		if err == nil {
			comment.User.ID = Ctx.User.ID
			comment.User = *Ctx.User
			if comment.ID > 0 {
				Ctx.Logger.WithField("msg", "Comment updates not supported yet").Info()
			} else {
				comment.ID = 0
				fmt.Println(frame.Data)
				err = Ctx.Db.Set("gorm:association_autoupdate", false).Save(&comment).Error
				//Ctx.Ws.WriteJSON()

				savedFrame := NewFrame(CommentSaveFrame, comment, frame.RequestID)

				for client := range *Ctx.Clients {
					err := client.WriteJSON(savedFrame)
					if err != nil {
						Ctx.Logger.WithError(err).Error("Unable to broadcast comment")
						client.Close()
						delete(*Ctx.Clients, client)
					}
				}
			}
			if err != nil {
				Ctx.Logger.WithError(err).Error("Unable to save comment")
			}
		} else {
			Ctx.Logger.WithError(err).Error("can't unmarshall comment")
		}
	} else {
		Ctx.Logger.WithField("msg", "Unauthorized comment save call").Info()
	}
}