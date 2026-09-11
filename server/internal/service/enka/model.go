package enka

// Enka.Network 原始响应结构（按真实返回值定义，只保留需要的字段）
// https://github.com/EnkaNetwork/API-docs

import "encoding/json"

type Profile struct {
	PlayerInfo     PlayerInfo   `json:"playerInfo"`
	AvatarInfoList []AvatarInfo `json:"avatarInfoList"`
	TTL            int          `json:"ttl"`
	UID            string       `json:"uid"`
	Region         string       `json:"region"`
}

type PlayerInfo struct {
	Nickname             string `json:"nickname"`
	Level                int    `json:"level"`
	Signature            string `json:"signature"`
	WorldLevel           int    `json:"worldLevel"`
	NameCardID           int    `json:"nameCardId"`
	FinishAchievementNum int    `json:"finishAchievementNum"`
	FetterCount          int    `json:"fetterCount"`
	ProfilePicture       struct {
		ID int `json:"id"`
	} `json:"profilePicture"`
}

type AvatarInfo struct {
	AvatarID     int                `json:"avatarId"`
	PropMap      map[string]PropVal `json:"propMap"`
	FightPropMap map[string]float64 `json:"fightPropMap"` // 面板属性（key 为 FIGHT_PROP 数字枚举）
	TalentIDList []int              `json:"talentIdList"` // 命座解锁列表（长度即命座数）
	// 天赋等级：key 为技能 id（升序依次对应 普攻 / 元素战技 / 元素爆发）
	SkillLevelMap map[string]json.Number `json:"skillLevelMap"`
	EquipList    []Equip            `json:"equipList"`
	FetterInfo   struct {
		ExpLevel int `json:"expLevel"`
	} `json:"fetterInfo"`
}

type PropVal struct {
	Type int    `json:"type"`
	IVal string `json:"ival"`
	Val  string `json:"val"`
}

// 角色等级的 propMap key（Enka 约定）
const propKeyLevel = "4001"

type Equip struct {
	ItemID    int              `json:"itemId"`
	Flat      EquipFlat        `json:"flat"`
	Weapon    *WeaponDetail    `json:"weapon,omitempty"`
	Reliquary *ReliquaryDetail `json:"reliquary,omitempty"`
}

type EquipFlat struct {
	ItemType             string      `json:"itemType"`             // ITEM_WEAPON / ITEM_RELIQUARY
	NameTextMapHash      json.Number `json:"nameTextMapHash"`      // 名称 hash（字符串大数）
	Icon                 string      `json:"icon"`
	RankLevel            int         `json:"rankLevel"`            // 星级
	EquipType            string      `json:"equipType"`            // 圣遗物部位（EQUIP_BRACER 等）
	SetID                int         `json:"setId"`                // 圣遗物套装 ID
	SetNameTextMapHash   json.Number `json:"setNameTextMapHash"`   // 套装名 hash
	ReliquarySubstats    []struct {
		AppendPropID string      `json:"appendPropId"` // FIGHT_PROP_XXX
		StatValue    float64     `json:"statValue"`
	} `json:"reliquarySubstats"`
	ReliquaryMainstat struct {
		MainPropID string  `json:"mainPropId"` // FIGHT_PROP_XXX
		StatValue  float64 `json:"statValue"`
	} `json:"reliquaryMainstat"`
	WeaponStats []struct {
		AppendPropID string  `json:"appendPropId"`
		StatValue    float64 `json:"statValue"`
	} `json:"weaponStats"`
}

type WeaponDetail struct {
	Level        json.Number `json:"level"`
	PromoteLevel json.Number `json:"promoteLevel"`
	AffixMap     map[string]json.Number `json:"affixMap"` // value 为精炼等级
}

type ReliquaryDetail struct {
	Level            json.Number `json:"level"`
	MainPropID       json.Number `json:"mainPropId"`
	AppendPropIDList []int64     `json:"appendPropIdList"` // 每个元素 = 一次获得/强化，id 对应词缀表
}
