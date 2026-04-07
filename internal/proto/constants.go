package proto

// Product signatures
const (
	ProductSigZone uint32 = 0x5A6F4E65 // 'ZoNe'
	ProductSigFree uint32 = 0x46524545 // 'FREE' (Millennium)
)

// Protocol signatures (used in AppHeader.Signature)
const (
	ProxySig       uint32 = 0x726F7574 // 'rout'
	GameRoomSig    uint32 = 0x67616D65 // 'game'
	LobbySig       uint32 = 0x6C626279 // 'lbby'
	CheckersSig    uint32 = 0x43484B52 // 'CHKR'
	InternalAppSig uint32 = 0x7A737973 // 'zsys'
)

// Internal connection-layer application message types.
const (
	ConnectionKeepAlive uint32 = 0x80000000
	ConnectionPing      uint32 = 0x80000001
	ConnectionPingReply uint32 = 0x80000002
)

// Protocol versions
const (
	ProxyVersion    uint32 = 1
	GameRoomVersion uint32 = 17
	CheckersVersion uint32 = 2
)

// Ports
const (
	PortZoneProxy       = 28803
	PortMillenniumProxy = 28805
)

// Proxy message types
const (
	ProxyMsgHi             uint16 = 0
	ProxyMsgHello          uint16 = 1
	ProxyMsgGoodbye        uint16 = 2
	ProxyMsgWrongVersion   uint16 = 3
	ProxyMsgServiceRequest uint16 = 4
	ProxyMsgServiceInfo    uint16 = 5
	ProxyNumBasicMessages  uint16 = 6
	ProxyMsgMillID         uint16 = 6 // = ProxyNumBasicMessages
	ProxyMsgMillSettings   uint16 = 7
)

// Proxy service request reasons
const (
	ProxyRequestInfo       uint32 = 0
	ProxyRequestConnect    uint32 = 1
	ProxyRequestDisconnect uint32 = 2
)

// Proxy service info reasons
const (
	ProxyServiceInfo       uint32 = 0
	ProxyServiceConnect    uint32 = 1
	ProxyServiceDisconnect uint32 = 2
)

// Proxy service flags
const (
	ProxyServiceAvailable uint32 = 0x01
	ProxyServiceLocal     uint32 = 0x02
	ProxyServiceConnected uint32 = 0x04
)

// Proxy Millennium chat settings
const (
	ProxyMillChatFull       uint16 = 1
	ProxyMillChatRestricted uint16 = 2
	ProxyMillChatNone       uint16 = 3
)

// Proxy Millennium stats settings
const (
	ProxyMillStatsAll     uint16 = 1
	ProxyMillStatsMost    uint16 = 2
	ProxyMillStatsMinimal uint16 = 3
)

// Room message types
const (
	RoomMsgUserInfo        uint32 = 0
	RoomMsgRoomInfo        uint32 = 1
	RoomMsgEnter           uint32 = 2
	RoomMsgLeave           uint32 = 3
	RoomMsgStartGame       uint32 = 5
	RoomMsgTableStatus     uint32 = 6
	RoomMsgTalkRequest     uint32 = 7
	RoomMsgTalkResponse    uint32 = 8
	RoomMsgGameMessage     uint32 = 9
	RoomMsgAccessed        uint32 = 11
	RoomMsgDisconnect      uint32 = 12
	RoomMsgSuspend         uint32 = 14
	RoomMsgLatency         uint32 = 17
	RoomMsgUserRatings     uint32 = 19
	RoomMsgZUserIDResponse uint32 = 23
	RoomMsgSeatRequest     uint32 = 24
	RoomMsgSeatResponse    uint32 = 25
	RoomMsgClearAllTables  uint32 = 26
	RoomMsgTableSettings   uint32 = 28
	RoomMsgClientConfig    uint32 = 30
	RoomMsgServerStatus    uint32 = 31
	RoomMsgStartGameM      uint32 = 32
	RoomMsgPing            uint32 = 64
)

// Seat actions
const (
	SeatActionSitDown        int16 = 0
	SeatActionLeaveTable     int16 = 1
	SeatActionStartGame      int16 = 2
	SeatActionReplacePlayer  int16 = 3
	SeatActionAddKibitzer    int16 = 4
	SeatActionRemoveKibitzer int16 = 5
	SeatActionJoin           int16 = 8
	SeatActionQuickHost      int16 = 9
	SeatActionQuickJoin      int16 = 10
	SeatActionDenied         int16 = 0x7FFF // 0x8000 as int16
)

// Table states
const (
	TableStateIdle      int16 = 0
	TableStateGaming    int16 = 1
	TableStateLaunching int16 = 2
)

// Game options
const (
	GameOptKibitzerAllowed  uint32 = 0x00000002
	GameOptJoiningAllowed   uint32 = 0x00000004
	GameOptRequiresFullTbl  uint32 = 0x00000008
	GameOptChatEnabled      uint32 = 0x00000010
	GameOptRatingsAvailable uint32 = 0x00000080
	GameOptStaticChat       uint32 = 0x00001000
	GameOptDynamicChat      uint32 = 0x00002000
)

// Checkers message types (inside RoomMsgGameMessage)
const (
	CheckersMsgNewGame       uint32 = 0x100
	CheckersMsgMovePiece     uint32 = 0x101
	CheckersMsgTalk          uint32 = 0x102
	CheckersMsgEndGame       uint32 = 0x103
	CheckersMsgEndLog        uint32 = 0x104
	CheckersMsgFinishMove    uint32 = 0x105
	CheckersMsgDraw          uint32 = 0x106
	CheckersMsgPlayers       uint32 = 0x107
	CheckersMsgGameStateReq  uint32 = 0x108
	CheckersMsgGameStateResp uint32 = 0x109
	CheckersMsgMoveTimeout   uint32 = 0x10A
	CheckersMsgVoteNewGame   uint32 = 0x10B
)

// Draw vote values
const (
	AcceptDraw int16 = 1
	RefuseDraw int16 = 2
)

// EndLog reasons
const (
	EndLogReasonTimeout  int16 = 1
	EndLogReasonForfeit  int16 = 2
	EndLogReasonWontPlay int16 = 3
	EndLogReasonGameOver int16 = 4
)

// String lengths
const (
	UserNameLen     = 31
	HostNameLen     = 16
	GameIDLen       = 31
	InternalNameLen = 15
	SetupTokenLen   = 63
)

// Max players
const (
	MaxPlayersPerTable = 8
	NumCheckersPlayers = 2
)
