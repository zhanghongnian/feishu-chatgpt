package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"start-feishubot/initialization"
	"start-feishubot/services"

	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkcontact "github.com/larksuite/oapi-sdk-go/v3/service/contact/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// 责任链
func chain(data *ActionInfo, actions ...Action) bool {
	for _, v := range actions {
		if !v.Execute(data) {
			return false
		}
	}
	return true
}

type MessageHandler struct {
	sessionCache services.SessionServiceCacheInterface
	msgCache     services.MsgCacheInterface
	gpt          *services.ChatGPT
	config       initialization.Config
}

func (m MessageHandler) cardHandler(_ context.Context,
	cardAction *larkcard.CardAction) (interface{}, error) {
	var cardMsg CardMsg
	actionValue := cardAction.Action.Value
	actionValueJson, _ := json.Marshal(actionValue)
	json.Unmarshal(actionValueJson, &cardMsg)
	if cardMsg.Kind == ClearCardKind {
		newCard, err, done := CommonProcessClearCache(cardMsg, m.sessionCache)
		if done {
			return newCard, err
		}
		return nil, nil
	}
	if cardMsg.Kind == PicResolutionKind {
		CommonProcessPicResolution(cardMsg, cardAction, m.sessionCache)
		return nil, nil
	}
	if cardMsg.Kind == PicMoreKind {
		go func() {
			m.CommonProcessPicMore(cardMsg)
		}()
	}

	return nil, nil

}

func (m MessageHandler) CommonProcessPicMore(msg CardMsg) {
	resolution := m.sessionCache.GetPicResolution(msg.SessionId)
	// fmt.Println("resolution: ", resolution)
	// fmt.Println("msg: ", msg)
	question := msg.Value.(string)
	bs64, _ := m.gpt.GenerateOneImage(question, resolution)
	replayImageByBase64(context.Background(), bs64, &msg.MsgId,
		&msg.SessionId, question)
}

func CommonProcessPicResolution(msg CardMsg,
	cardAction *larkcard.CardAction,
	cache services.SessionServiceCacheInterface) {
	option := cardAction.Action.Option
	// fmt.Println(larkcore.Prettify(msg))
	cache.SetPicResolution(msg.SessionId, services.Resolution(option))
	// send text
	replyMsg(context.Background(), "已更新图片分辨率为"+option,
		&msg.MsgId)
}

func CommonProcessClearCache(cardMsg CardMsg, session services.SessionServiceCacheInterface) (
	interface{}, error, bool) {
	if cardMsg.Value == "1" {
		newCard, _ := newSendCard(
			withHeader("️🆑 机器人提醒", larkcard.TemplateRed),
			withMainMd("已删除此话题的上下文信息"),
			withNote("我们可以开始一个全新的话题，继续找我聊天吧"),
		)
		session.Clear(cardMsg.SessionId)
		return newCard, nil, true
	}
	if cardMsg.Value == "0" {
		newCard, _ := newSendCard(
			withHeader("️🆑 机器人提醒", larkcard.TemplateGreen),
			withMainMd("依旧保留此话题的上下文信息"),
			withNote("我们可以继续探讨这个话题,期待和您聊天。如果您有其他问题或者想要讨论的话题，请告诉我哦"),
		)
		return newCard, nil, true
	}
	return nil, nil, false
}

func (m MessageHandler) msgReceivedHandler(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	handlerType := judgeChatType(event)
	if handlerType == "otherChat" {
		fmt.Println("unknown chat type")
		return nil
	}
	msgType := judgeMsgType(event)
	if msgType != "text" && msgType != "audio" {
		fmt.Println("unknown msg type")
		return nil
	}
	fmt.Println("")
	fmt.Println(larkcore.Prettify(event.Event.Sender))
	client := initialization.GetLarkClient()
	req := larkcontact.NewGetUserReqBuilder().
		UserId(*event.Event.Sender.SenderId.OpenId).
		Build()
	resp, err := client.Contact.User.Get(ctx, req)
	if err != nil {
		fmt.Println(err)
	} else {
		// fmt.Println(larkcore.Prettify(resp))
		fmt.Println(*resp.Data.User.Name)
	}
	// fmt.Println(larkcore.Prettify(event.Event.Message))

	content := event.Event.Message.Content
	msgId := event.Event.Message.MessageId
	rootId := event.Event.Message.RootId
	chatId := event.Event.Message.ChatId
	mention := event.Event.Message.Mentions

	sessionId := rootId
	if sessionId == nil || *sessionId == "" {
		sessionId = msgId
	}
	msgInfo := MsgInfo{
		openId:      event.Event.Sender.SenderId.OpenId,
		handlerType: handlerType,
		msgType:     msgType,
		msgId:       msgId,
		chatId:      chatId,
		qParsed:     strings.Trim(parseContent(*content), " "),
		fileKey:     parseFileKey(*content),
		sessionId:   sessionId,
		mention:     mention,
	}
	data := &ActionInfo{
		ctx:     &ctx,
		handler: &m,
		info:    &msgInfo,
	}
	actions := []Action{
		&ProcessedUniqueAction{}, // 避免重复处理
		&ProcessMentionAction{},  // 判断机器人是否应该被调用
		&AudioAction{},           // 语音处理
		&EmptyAction{},           // 空消息处理
		&ClearAction{},           // 清除消息处理
		&HelpAction{},            // 帮助处理
		&RolePlayAction{},        // 角色扮演处理
		&PicAction{},             // 图片处理
		&MessageAction{},         // 消息处理

	}
	chain(data, actions...)
	return nil
}

var _ MessageHandlerInterface = (*MessageHandler)(nil)

func NewMessageHandler(gpt *services.ChatGPT,
	config initialization.Config) MessageHandlerInterface {
	return &MessageHandler{
		sessionCache: services.GetSessionCache(),
		msgCache:     services.GetMsgCache(),
		gpt:          gpt,
		config:       config,
	}
}

func (m MessageHandler) judgeIfMentionMe(mention []*larkim.
	MentionEvent) bool {
	if len(mention) != 1 {
		return false
	}
	return *mention[0].Name == m.config.FeishuBotName
}
