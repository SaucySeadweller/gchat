package storage

import (
	"log"

	"github.com/hibooboo2/gchat/api"
	"github.com/hibooboo2/gchat/server/model"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (d *DB) AllFriends(username string) (*api.FriendsList, error) {
	friends := []model.Friend{}
	err := d.db.Find(&friends, `user_a = ? or user_b = ?`, username, username).Error
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get friends from database: %v", err.Error())
	}
	friendListNames := []string{}
	fl := &api.FriendsList{Friends: []*api.Friend{}}
	for _, friend := range friends {
		if friend.UserA == username {
			friendListNames = append(friendListNames, friend.UserB)
		} else {
			friendListNames = append(friendListNames, friend.UserA)
		}
	}

	users := []*api.Friend{}
	err = d.db.Table(model.User{}.TableName()).Select([]string{"status", "username", "is_online as online"}).Find(&users, `username in (?)`, friendListNames).Error
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get friends for %s", username)
	}
	fl.Friends = users
	return fl, nil
}

func (d *DB) AddFriend(usernameA string, usernameB string) error {
	usrs := []string{usernameA, usernameB}

	friend := &api.Friend{}
	err := d.db.First(&friend, `(user_a = ? AND user_b = ?) OR (user_b = ? and user_a = ?)`, usernameA, usernameB, usernameA, usernameB).Error
	if err != nil && !gorm.IsRecordNotFoundError(err) {
		return status.Errorf(codes.Internal, "failed to get from friends: %v", err.Error())
	}

	if !gorm.IsRecordNotFoundError(err) {
		return status.Errorf(codes.AlreadyExists, "You are already friends with %s", usernameB)
	}

	req := model.FriendRequest{}
	err = d.db.First(&req, `requestor in (?) and user in (?)`, usrs, usrs).Error
	if err != nil && !gorm.IsRecordNotFoundError(err) {
		return status.Errorf(codes.Internal, "failed to get from friend requests: %v", err.Error())
	}

	f := model.Friend{}
	f.UserA = usernameA
	f.UserB = usernameB

	if gorm.IsRecordNotFoundError(err) {
		err = d.db.Save(&model.FriendRequest{Requestor: usernameA, User: usernameB}).Error
		if err != nil {
			return status.Errorf(codes.Internal, "failed to save friend request: %v", err)
		}
		log.Printf("trace: friend request saved %s %s", usernameA, usernameB)
		return nil
	}

	if req.Requestor == usernameA {
		return status.Errorf(codes.AlreadyExists, "There is already a friend request to %s from %s", usernameB, usernameA)
	}

	err = d.db.Unscoped().Delete(req).Error
	if err != nil {
		return status.Errorf(codes.Internal, "failed to delete friend request: %v", err.Error())
	}
	err = d.db.Save(&f).Error
	if err != nil {
		return status.Errorf(codes.Internal, "failed to save friend: %v", err.Error())
	}
	return nil
}

func (d *DB) RemoveFriend(usernameA string, usernameB string) error {
	usrs := []string{usernameA, usernameB}
	err := d.db.Debug().Exec(`DELETE FROM friends WHERE user_a in (?) and user_b in (?)`, usrs, usrs).Error
	if gorm.IsRecordNotFoundError(err) {
		err := d.db.Exec(`DELETE FROM friend_requests WHERE user_a in (?) and user_b in (?)`, usrs, usrs).Error
		if gorm.IsRecordNotFoundError(err) {
			return status.Errorf(codes.NotFound, "you are not friends with user: %s", usernameB)

		}
		return status.Errorf(codes.Internal, "failed to remove friend: %v", err.Error())
	}
	if err != nil {
		return status.Errorf(codes.Internal, "failed to remove friend: %v", err.Error())
	}
	return nil
}

func (d *DB) GetFriendRequests(username string) (*api.FriendRequests, error) {
	reqs := []model.FriendRequest{}
	err := d.db.Raw(`SELECT * from friend_requests where user = ?`, username).Scan(&reqs).Error
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get friend requests: %v", err.Error())
	}
	fr := &api.FriendRequests{}
	for _, req := range reqs {
		fr.Friends = append(fr.Friends, &api.Friend{Username: req.Requestor})
	}
	return fr, nil
}
