// draw-image-plugin 派系 & Boss 枚举
package drawplugin

type FactionInfo struct {
	Name  string
	Color string
	Icon  string
}

type Faction string

const (
	FactionGrineer   Faction = "FC_GRINEER"
	FactionCorpus    Faction = "FC_CORPUS"
	FactionInfest    Faction = "FC_INFESTATION"
	FactionOrokin    Faction = "FC_OROKIN"
	FactionCorrupted Faction = "FC_CORRUPTED"
	FactionSentient  Faction = "FC_SENTIENT"
	FactionNarmer    Faction = "FC_NARMER"
	FactionMurmur    Faction = "FC_MURMUR"
	FactionScaldra   Faction = "FC_SCALDRA"
	FactionTechrot   Faction = "FC_TECHROT"
	FactionDuviri    Faction = "FC_DUVIRI"
	FactionMitw      Faction = "FC_MITW"
	FactionTenno     Faction = "FC_TENNO"
	FactionCrossfire Faction = "FC_CROSSFIRE"
	FactionNone      Faction = "FC_NONE"
)

var FactionMap = map[Faction]FactionInfo{
	FactionGrineer:   {"Grineer", "#870507", "\ue401"},
	FactionCorpus:    {"Corpus", "#5AB0CB", "\ue402"},
	FactionInfest:    {"Infested", "#61814B", "\ue403"},
	FactionOrokin:    {"奥罗金", "#B6A019", "\ue404"},
	FactionCorrupted: {"堕落者", "#B6A019", "\ue404"},
	FactionSentient:  {"Sentient", "#931B1B", "\ue405"},
	FactionNarmer:    {"合一众", "#A35F27", "\ue409"},
	FactionMurmur:    {"低语者", "#B59B6D", "\ue410"},
	FactionScaldra:   {"炽蛇军", "#DC8E12", "\ue411"},
	FactionTechrot:   {"科腐者", "#17A36A", "\ue412"},
	FactionDuviri:    {"双衍王境", "#000000", ""},
	FactionMitw:      {"墙中人", "#000000", ""},
	FactionTenno:     {"TENNO", "#000000", "\ue400"},
	FactionCrossfire: {"多方交战", "#591A9C", "\ue413"},
	FactionNone:      {"未知派系", "#000000", ""},
}

type BossInfo struct {
	Name    string
	Faction string
}

type Boss string

const (
	BossHyena        Boss = "SORTIE_BOSS_HYENA"
	BossKela         Boss = "SORTIE_BOSS_KELA"
	BossVor          Boss = "SORTIE_BOSS_VOR"
	BossRuk          Boss = "SORTIE_BOSS_RUK"
	BossHek          Boss = "SORTIE_BOSS_HEK"
	BossKril         Boss = "SORTIE_BOSS_KRIL"
	BossTyl          Boss = "SORTIE_BOSS_TYL"
	BossJackal       Boss = "SORTIE_BOSS_JACKAL"
	BossAlad         Boss = "SORTIE_BOSS_ALAD"
	BossAmbus        Boss = "SORTIE_BOSS_AMBULAS"
	BossNef          Boss = "SORTIE_BOSS_NEF"
	BossRaptor       Boss = "SORTIE_BOSS_RAPTOR"
	BossPhorid       Boss = "SORTIE_BOSS_PHORID"
	BossLephantis    Boss = "SORTIE_BOSS_LEPHANTIS"
	BossInfAlad      Boss = "SORTIE_BOSS_INFALAD"
	BossCorruptedVor Boss = "SORTIE_BOSS_CORRUPTED_VOR"
	BossBoreal       Boss = "SORTIE_BOSS_BOREAL"
	BossAmar         Boss = "SORTIE_BOSS_AMAR"
	BossNira         Boss = "SORTIE_BOSS_NIRA"
	BossPaazul       Boss = "SORTIE_BOSS_PAAZUL"
)

var BossMap = map[Boss]BossInfo{
	BossHyena:        {"鬣狗群", "Corpus"},
	BossKela:         {"Kela De Thaym", "Grineer"},
	BossVor:          {"Captian Vor", "Grineer"},
	BossRuk:          {"Lech Kril", "Grineer"},
	BossHek:          {"Councilor Vay Hek", "Grineer"},
	BossKril:         {"Lech Kril", "Grineer"},
	BossTyl:          {"Tyl Regor", "Grineer"},
	BossJackal:       {"豺狼", "Corpus"},
	BossAlad:         {"Alad V", "Corpus"},
	BossAmbus:        {"Ambulas", "Corpus"},
	BossNef:          {"Nef Anyo", "Corpus"},
	BossRaptor:       {"猛禽", "Corpus"},
	BossPhorid:       {"Phorid", "Infestation"},
	BossLephantis:    {"Lephantis", "Infestation"},
	BossInfAlad:      {"Mutalist Alad V", "Infestation"},
	BossCorruptedVor: {"堕落 Vor", "堕落者"},
	BossBoreal:       {"Boreal", "合一众"},
	BossAmar:         {"Amar", "合一众"},
	BossNira:         {"Nira", "合一众"},
	BossPaazul:       {"Paazul", "合一众"},
}
