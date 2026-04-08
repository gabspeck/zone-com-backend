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
	ReversiSig     uint32 = 0x72767365 // 'rvse'
	BackgammonSig  uint32 = 0x474B4342 // 'BCKG'
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
	ProxyVersion      uint32 = 1
	GameRoomVersion   uint32 = 17
	CheckersVersion   uint32 = 2
	ReversiVersion    uint32 = 3
	BackgammonVersion uint32 = 3
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
	RoomMsgChatSwitch      uint32 = 33
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

// Reversi message types (inside RoomMsgGameMessage)
const (
	ReversiMsgNewGame       uint32 = 0x100
	ReversiMsgMovePiece     uint32 = 0x101
	ReversiMsgTalk          uint32 = 0x102
	ReversiMsgEndGame       uint32 = 0x103
	ReversiMsgEndLog        uint32 = 0x104
	ReversiMsgFinishMove    uint32 = 0x105
	ReversiMsgPlayers       uint32 = 0x106
	ReversiMsgGameStateReq  uint32 = 0x107
	ReversiMsgGameStateResp uint32 = 0x108
	ReversiMsgMoveTimeout   uint32 = 0x109
	ReversiMsgVoteNewGame   uint32 = 0x10A
)

// Shared generic game message types.
const (
	GameMsgCheckIn           uint32 = 1024
	GameMsgReplacePlayer     uint32 = 1025
	GameMsgTableOptions      uint32 = 1026
	GameMsgGameStateRequest  uint32 = 1027
	GameMsgGameStateResponse uint32 = 1028
)

// Backgammon message types.
const (
	BackgammonMsgTalk           uint32 = 0x100
	BackgammonMsgTransaction    uint32 = 0x101
	BackgammonMsgTurnNotation   uint32 = 0x102
	BackgammonMsgTimestamp      uint32 = 0x103
	BackgammonMsgSavedGameState uint32 = 0x104
	BackgammonMsgRollRequest    uint32 = 0x105
	BackgammonMsgDiceRoll       uint32 = 0x106
	BackgammonMsgEndLog         uint32 = 0x107
	BackgammonMsgNewMatch       uint32 = 0x108
	BackgammonMsgFirstMove      uint32 = 0x109
	BackgammonMsgMoveTimeout    uint32 = 0x10A
	BackgammonMsgEndTurn        uint32 = 0x10B
	BackgammonMsgEndGame        uint32 = 0x10C
	BackgammonMsgGoFirstRoll    uint32 = 0x10D
	BackgammonMsgTieRoll        uint32 = 0x10E
	BackgammonMsgCheater        uint32 = 0x10F
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
	MaxPlayersPerTable   = 8
	NumCheckersPlayers   = 2
	NumReversiPlayers    = 2
	NumBackgammonPlayers = 2
	NumHeartsPlayers     = 4
)

// Hearts protocol
const (
	HeartsSig     uint32 = 0x6872747A // 'hrtz'
	HeartsVersion uint32 = 5
)

// Hearts message types (inside RoomMsgGameMessage)
const (
	HeartsMsgStartGame       uint32 = 0x100
	HeartsMsgReplacePlayer   uint32 = 0x101
	HeartsMsgStartHand       uint32 = 0x102
	HeartsMsgStartPlay       uint32 = 0x103
	HeartsMsgEndHand         uint32 = 0x104
	HeartsMsgEndGame         uint32 = 0x105
	HeartsMsgClientReady     uint32 = 0x106
	HeartsMsgPassCards       uint32 = 0x107
	HeartsMsgPlayCard        uint32 = 0x108
	HeartsMsgNewGame         uint32 = 0x109
	HeartsMsgTalk            uint32 = 0x10A
	HeartsMsgGameStateReq    uint32 = 0x10B
	HeartsMsgGameStateResp   uint32 = 0x10C
	HeartsMsgDumpHand        uint32 = 0x10D
	HeartsMsgOptions         uint32 = 0x10E
	HeartsMsgCheckIn         uint32 = 0x10F
	HeartsMsgRemovePlayerReq uint32 = 0x110
	HeartsMsgRemovePlayerRes uint32 = 0x111
	HeartsMsgDossierData     uint32 = 0x112
	HeartsMsgDossierVote     uint32 = 0x113
	HeartsMsgCloseRequest    uint32 = 0x114
	HeartsMsgCloseDenied     uint32 = 0x115
)

// Hearts constants
const (
	HeartsMaxNumPlayers     = 6
	HeartsMaxNumCardsInHand = 18
	HeartsMaxNumCardsInPass = 5
	HeartsNumCardsInDeck    = 52
	HeartsNumPointsInGame   = 100
	HeartsCardNone          = 127
)

// Hearts pass directions
const (
	HeartsPassHold   int16 = 0
	HeartsPassLeft   int16 = 1
	HeartsPassAcross int16 = 2
	HeartsPassRight  int16 = 3
)

// Hearts server states
const (
	HeartsStateNone      int16 = 0
	HeartsStatePassCards int16 = 1
	HeartsStatePlayCards int16 = 2
	HeartsStateEndGame   int16 = 3
)

// Spades protocol
const (
	SpadesSig     uint32 = 0x7368766C // 'shvl'
	SpadesVersion uint32 = 4
)

// Spades message types (inside RoomMsgGameMessage)
const (
	SpadesMsgClientReady        uint32 = 0x100
	SpadesMsgStartGame          uint32 = 0x101
	SpadesMsgReplacePlayer      uint32 = 0x102
	SpadesMsgStartBid           uint32 = 0x103
	SpadesMsgStartPass          uint32 = 0x104 // disabled in XP
	SpadesMsgStartPlay          uint32 = 0x105
	SpadesMsgEndHand            uint32 = 0x106
	SpadesMsgEndGame            uint32 = 0x107
	SpadesMsgBid                uint32 = 0x108
	SpadesMsgPass               uint32 = 0x109 // disabled in XP
	SpadesMsgPlay               uint32 = 0x10A
	SpadesMsgNewGame            uint32 = 0x10B
	SpadesMsgTalk               uint32 = 0x10C
	SpadesMsgGameStateReq       uint32 = 0x10D
	SpadesMsgGameStateResp      uint32 = 0x10E
	SpadesMsgOptions            uint32 = 0x10F
	SpadesMsgCheckIn            uint32 = 0x110
	SpadesMsgTeamName           uint32 = 0x111
	SpadesMsgRemovePlayerReq    uint32 = 0x112
	SpadesMsgRemovePlayerResp   uint32 = 0x113
	SpadesMsgRemovePlayerEnd    uint32 = 0x114
	SpadesMsgDossierData        uint32 = 0x115
	SpadesMsgDossierVote        uint32 = 0x116
	SpadesMsgShownCards         uint32 = 0x117
	SpadesMsgDumpHand           uint32 = 0x400
)

// Spades constants
const (
	SpadesNumPlayers     = 4
	SpadesNumTeams       = 2
	SpadesNumCardsInHand = 13
)

// Spades server states
const (
	SpadesStateNone    int16 = 0
	SpadesStateBidding int16 = 1
	SpadesStatePlaying int16 = 3
	SpadesStateEndHand int16 = 4
	SpadesStateEndGame int16 = 5
)

