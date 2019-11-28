package main

import (
	"context"
	"fmt"
	"log"

	"github.com/hibooboo2/gchat/api"
	"github.com/hibooboo2/gchat/utils"
	"github.com/rivo/tview"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func main() {
	log.SetFlags(log.Lshortfile)
	// dail server
	conn, err := grpc.Dial(":9090", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("can not connect with server %v", err)
	}

	// create stream
	chatClient := api.NewChatClient(conn)
	authClient := api.NewAuthClient(conn)
	friendsClient := api.NewFriendsClient(conn)

	es := &ExecutorScope{authClient: authClient, chatClient: chatClient, friendClient: friendsClient}
	es.ui()
}

type ExecutorScope struct {
	authClient   api.AuthClient
	ctx          context.Context
	chatClient   api.ChatClient
	friendClient api.FriendsClient
	friendList   map[string]*api.Friend
	app          *tview.Application
	list         *tview.List
	flexx        *tview.Flex
	friendTree   *tview.TreeView
}

func (e *ExecutorScope) login(username string, pass string) error {
	in := api.LoginRequest{
		Username: username,
		Password: pass,
	}

	in.Password = utils.Hash(in.Password)

	ctx := context.Background()
	l, err := e.authClient.Login(ctx, &in)
	if err != nil {
		return err
	}
	e.ctx = metadata.AppendToOutgoingContext(ctx, "TOKEN", l.Token)

	return nil
}

func (e *ExecutorScope) sendFriendRequest(elementToFocus tview.Primitive) {
	req := api.Friend{}

	form := tview.NewForm()
	form.AddInputField("username", "", 10, tview.InputFieldMaxLength(100), func(text string) {
		req.Username = text
	})
	_, err := e.friendClient.Add(e.ctx, &req)
	if err != nil {
		e.modal(e.list, err.Error())
	}
	form.AddButton("send friend request", func() {
		_, err := e.friendClient.Add(e.ctx, &req)
		if err != nil {
			e.modal(form, err.Error())
			return
		}
		e.app.SetRoot(elementToFocus, true).SetFocus(elementToFocus).Draw()

	})
	e.app.SetRoot(form, true).SetFocus(form).Draw()
}

func (e *ExecutorScope) removeFriend(elementToFocus tview.Primitive) {
	username := ""
	req := api.Friend{}
	form := tview.NewForm()
	form.AddInputField("username", "", 10, tview.InputFieldMaxLength(100), func(text string) {
		req.Username = text
	})
	_, err := e.friendClient.Remove(e.ctx, &req)
	if err != nil {
		e.modal(form, err.Error())

	}

	form.AddButton("remove friend", func() {
		_, err := e.friendClient.Remove(e.ctx, &req)
		if err != nil {
			e.modal(form, err.Error())
			return
		}

		e.app.SetRoot(elementToFocus, true).SetFocus(elementToFocus).Draw()

	})

	delete(e.friendList, username)
	e.app.SetRoot(form, true).SetFocus(form).Draw()
}

func (e *ExecutorScope) getFriends(elementToFocus tview.Primitive) {
	e.friendList = map[string]*api.Friend{}
	friends, err := e.friendClient.All(e.ctx, &api.FriendsListReq{})
	if err != nil {
		e.modal(e.app.GetFocus(), err.Error())
	}
	e.friendTree = tview.NewTreeView()
	friendRoot := tview.NewTreeNode("friends")
	e.friendTree.SetRoot(friendRoot)
	for _, friend := range friends.Friends {
		e.friendList[friend.Username] = friend
		friendRoot.AddChild(tview.NewTreeNode(friend.Username))
	}
	e.flexx.AddItem(e.friendTree, 0, 1, false)
	e.friendTree.SetBorder(true)

}

func (e *ExecutorScope) status() {
	stream, err := e.friendClient.Status(e.ctx, &api.Empty{})
	if err != nil {
		fmt.Println(err)
	}
	go func() {
		for {
			status, err := stream.Recv()
			if err != nil {
				fmt.Println(err)
				break
			}
			friend, ok := e.friendList[status.Username]
			if ok {
				friend.Status = status.Status
			}
		}
	}()
}

func (e *ExecutorScope) ui() {
	e.app = tview.NewApplication()
	list := tview.NewList()
	e.list = list
	list.AddItem("register", "register to use the program", 'r', func() { e.registerScreen(e.flexx) })
	list.AddItem("login", "login as a user", 'l', func() { e.loginScreen(e.flexx) })
	list.AddItem("quit", "quit the program", 'q', func() { e.app.Stop() })

	e.flexx = tview.NewFlex()
	e.flexx.SetDirection(tview.FlexColumn)
	e.flexx.AddItem(list, 0, 1, true)
	e.flexx.SetTitle("Gchat")
	e.flexx.SetBorder(true)
	if err := e.app.SetRoot(e.flexx, true).Run(); err != nil {
		panic(err)
	}

}

func (e *ExecutorScope) registerScreen(elementToFocus tview.Primitive) {
	req := api.RegisterRequest{}
	form := tview.NewForm()
	form.AddInputField("Email", "", 10, tview.InputFieldMaxLength(100), func(text string) {
		req.Email = text
	})

	form.AddInputField("Username", "", 10, tview.InputFieldMaxLength(100), func(text string) {
		req.Username = text
	})
	form.AddPasswordField("Password", "", 10, '*', func(text string) {
		req.Password = text
	})
	form.AddInputField("First Name", "", 10, tview.InputFieldMaxLength(100), func(text string) {
		req.FirstName = text
	})
	form.AddInputField("Last Name", "", 10, tview.InputFieldMaxLength(100), func(text string) {
		req.LastName = text
	})

	form.AddButton("Register", func() {
		_, err := e.authClient.Register(context.Background(), &req)
		if err != nil {
			e.modal(form, err.Error())
			return
		}
		e.app.SetRoot(elementToFocus, true).SetFocus(elementToFocus).Draw()
	})
	e.app.SetRoot(form, true).SetFocus(form).Draw()

}

func (e *ExecutorScope) loginScreen(elementToFocus tview.Primitive) {
	form := tview.NewForm()
	username := ""
	form.AddInputField("Username", "", 10, tview.InputFieldMaxLength(100), func(text string) {
		username = text
	})
	pass := ""
	form.AddPasswordField("Password", "", 10, '*', func(text string) {
		pass = text
	})

	form.AddButton("Login", func() {
		err := e.login(username, pass)
		if err != nil {
			e.modal(form, err.Error())
			return
		}
		e.list.RemoveItem(e.list.GetCurrentItem() - 1)
		e.list.RemoveItem(e.list.GetCurrentItem())
		e.list.RemoveItem(e.list.GetCurrentItem())
		e.app.SetFocus(elementToFocus).SetRoot(elementToFocus, true).Draw()
		e.list.AddItem("send message", "send a message to a user", 'm', func() { e.messageScreen(e.flexx) })
		e.list.AddItem("add friend", "add a friend", 'f', func() { e.sendFriendRequest(e.flexx) })
		e.list.AddItem("remove friend", "remove a friend", 'r', func() { e.removeFriend(e.flexx) })
		e.list.AddItem("quit", "quit the program", 'q', func() { e.app.Stop() })
		e.getFriends(elementToFocus)
		go e.notificationScreen(e.flexx)

	})

	e.app.SetRoot(form, true).SetFocus(form).Draw()
}

func (e *ExecutorScope) messageScreen(elementToFocus tview.Primitive) {
	req := api.Message{}
	form := tview.NewForm()
	form.AddInputField("Username", "", 10, tview.InputFieldMaxLength(100), func(text string) {
		req.To = text
	})
	form.AddInputField("Message", "", 10, tview.InputFieldMaxLength(100), func(text string) {
		req.Data = text

	})
	form.AddButton("send message", func() {
		_, err := e.chatClient.SendMessage(e.ctx, &req)
		if err != nil {
			e.modal(form, err.Error())
		}
		e.app.SetRoot(elementToFocus, true).SetFocus(elementToFocus).Draw()
	})
	e.app.SetRoot(form, true).SetFocus(form).Draw()
}

func (e *ExecutorScope) modal(elementToFocus tview.Primitive, s string) {
	m := tview.NewModal()
	m.AddButtons([]string{"Ok"}).SetText(s)
	m.SetDoneFunc(func(num int, s string) {
		e.app.SetRoot(elementToFocus, true).SetFocus(elementToFocus).Draw()
	})
	e.app.SetRoot(m, true).SetFocus(m).Draw()

}

func (e *ExecutorScope) notificationScreen(elementToFocus tview.Primitive) {
	msgClient, err := e.chatClient.Messages(e.ctx, &api.Empty{})
	if err != nil {
		e.modal(e.app.GetFocus(), err.Error())
		return
	}
	pages := tview.NewPages()
	pages.SetBorder(true)
	e.flexx.AddItem(pages, 0, 1, false)
	userMessages := map[string]*tview.TextView{}
	for {
		message, err := msgClient.Recv()
		if err != nil {
			e.modal(e.app.GetFocus(), err.Error())
			return
		}

		if !pages.HasPage(message.From) {
			tv := tview.NewTextView()
			userMessages[message.From] = tv
			pages.AddPage(message.From, tv, true, true)
		}
		fmt.Fprintf(userMessages[message.From], "%v, %v", message.Data, message.From)
		pages.SwitchToPage(message.From)
	}

}
