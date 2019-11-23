package main

import (
	"context"
	"fmt"
	"log"

	prompt "github.com/c-bata/go-prompt"
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
}

func (e *ExecutorScope) executor(t string) {
	var err error
	fmt.Println("You selected " + t)
	switch t {
	case "register":
		err = reg(e.authClient)
	case "notifications":
		e.messageNotifications()
	case "send friend request":
		e.sendFriendRequest()
	case "friends list":
		e.getFriends()
	case "remove friend":
		e.removeFriend()
	case "status":
		e.status()
	}
	if err != nil {
		fmt.Println(err)
	}

}

func reg(authClient api.AuthClient) error {
	ctx := context.Background()
	req := &api.RegisterRequest{}
	req.Email = prompt.Input("What is your email?", Empty)
	req.Username = prompt.Input("What is your desired username?", Empty)
	req.Password = prompt.Input("What do you want to set your password as?", Empty)
	req.FirstName = prompt.Input("(Optional)What is your first name ?", Empty)
	req.LastName = prompt.Input("(Optional)What is your last name?", Empty)
	regResp, err := authClient.Register(ctx, req)
	if err != nil {
		return err
	}
	log.Println(regResp)
	return nil
}

func Empty(d prompt.Document) []prompt.Suggest { return nil }

func Commands(d prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{
		{Text: "register", Description: "Register a user"},
		{Text: "login", Description: "Login a user"},
		{Text: "message", Description: "Send a message to a user"},
		{Text: "notifications", Description: "Pull up notifications"},
		{Text: "send friend request", Description: "Send a user a friend request"},
		{Text: "friends list", Description: "Get a list of your friends"},
		{Text: "remove friend", Description: "Removes a friend from your friends list"},
		{Text: "status", Description: "checks the status of friends  from your friends list"},
	}

	return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
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

func (e *ExecutorScope) messageNotifications() {
	stream, err := e.chatClient.Messages(e.ctx, &api.Empty{})
	if err != nil {
		fmt.Println(err)
		return
	}
	go func() {
		msg, err := stream.Recv()
		for err == nil {
			if err == nil {
				fmt.Println(msg.From, msg.Data)
			}
			msg, err = stream.Recv()
		}
	}()
}

func (e *ExecutorScope) sendFriendRequest() {
	_, err := e.friendClient.Add(e.ctx, &api.Friend{
		Username: prompt.Input("Who do you want to send a friend request to?", Empty),
	})
	if err != nil {
		fmt.Println(err)
	}
}

func (e *ExecutorScope) getFriends() {
	e.friendList = map[string]*api.Friend{}
	friends, err := e.friendClient.All(e.ctx, &api.FriendsListReq{})
	if err != nil {
		fmt.Println(err)
	}
	for _, friend := range friends.Friends {
		e.friendList[friend.Username] = friend
	}

}

func (e *ExecutorScope) removeFriend() {
	username := prompt.Input("Which friend do you want to delete?", Empty)
	_, err := e.friendClient.Remove(e.ctx, &api.Friend{
		Username: username,
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	delete(e.friendList, username)

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
	list.AddItem("register", "register to use the program", 'r', func() { e.registerScreen(list) })
	list.AddItem("login", "login as a user", 'l', func() { e.loginScreen(list) })
	list.AddItem("send message", "send a message to a user", 'm', func() { e.messageScreen(list) })
	list.AddItem("quit", "quit the program", 'q', func() { e.app.Stop() })

	if err := e.app.SetRoot(list, true).SetFocus(list).Run(); err != nil {
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
		e.modal(elementToFocus, "Logged in as "+username)
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
