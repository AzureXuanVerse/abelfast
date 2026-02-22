package playerops

import (
	"encoding/json"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
)

const (
	oilResourceID  = 2
	coinResourceID = 1
)

type oilfieldRuntimeTemplate struct {
	Level      uint32 `json:"level"`
	Production uint32 `json:"production"`
	Store      uint32 `json:"store"`
	HourTime   uint32 `json:"hour_time"`
	Time       uint32 `json:"time"`
}

type academyRuntimeTemplates struct {
	oilByLevel map[uint32]oilfieldRuntimeTemplate
	maxLevel   uint32
}

func applyNavalAcademyLoginCatchup(client *connection.Client, now time.Time) error {
	if client.Commander.OwnedResourcesMap == nil {
		if err := client.Commander.Load(); err != nil {
			return err
		}
	}

	templates, err := loadAcademyRuntimeTemplates()
	if err != nil {
		return err
	}

	runtime, err := orm.LoadOrCreateNavalAcademyRuntime(client.Commander.CommanderID)
	if err != nil {
		return err
	}

	nowUnix := uint32(now.Unix())
	baseline := nowUnix
	if !client.PreviousLoginAt.IsZero() && client.PreviousLoginAt.Unix() > 0 {
		baseline = uint32(client.PreviousLoginAt.Unix())
	}
	if baseline > nowUnix {
		baseline = nowUnix
	}

	runtimeChanged := normalizeNavalAcademyRuntime(runtime, templates, nowUnix, baseline)

	oilTemplate := templates.oilByLevel[runtime.OilWellLevel]
	oilGain := computeFacilityAccrual(
		runtime.OilCollectTimestamp,
		nowUnix,
		runtime.OilUpgradeStartTime,
		runtime.OilUpgradeCompleteTime,
		oilTemplate,
	)
	if oilGain > 0 {
		if err := client.Commander.AddResource(oilResourceID, oilGain); err != nil {
			return err
		}
	}
	runtime.OilCollectTimestamp = nowUnix

	coinTemplate := templates.oilByLevel[runtime.GoldWellLevel]
	coinGain := computeFacilityAccrual(
		runtime.GoldCollectTimestamp,
		nowUnix,
		runtime.GoldUpgradeStartTime,
		runtime.GoldUpgradeCompleteTime,
		coinTemplate,
	)
	if coinGain > 0 {
		if err := client.Commander.AddResource(coinResourceID, coinGain); err != nil {
			return err
		}
	}
	runtime.GoldCollectTimestamp = nowUnix

	if runtimeChanged || oilGain > 0 || coinGain > 0 {
		if err := orm.SaveNavalAcademyRuntime(runtime); err != nil {
			return err
		}
	}

	return nil
}

func loadNavalAcademyRuntimeSnapshot(commanderID uint32, now time.Time) (*orm.NavalAcademyRuntime, error) {
	templates, err := loadAcademyRuntimeTemplates()
	if err != nil {
		return nil, err
	}
	runtime, err := orm.LoadOrCreateNavalAcademyRuntime(commanderID)
	if err != nil {
		return nil, err
	}
	if normalizeNavalAcademyRuntime(runtime, templates, uint32(now.Unix()), 0) {
		if err := orm.SaveNavalAcademyRuntime(runtime); err != nil {
			return nil, err
		}
	}
	return runtime, nil
}

func loadAcademyRuntimeTemplates() (*academyRuntimeTemplates, error) {
	entries, err := orm.ListConfigEntries("ShareCfg/oilfield_template.json")
	if err != nil {
		return nil, err
	}

	templates := &academyRuntimeTemplates{oilByLevel: make(map[uint32]oilfieldRuntimeTemplate)}
	for _, entry := range entries {
		var row oilfieldRuntimeTemplate
		if err := json.Unmarshal(entry.Data, &row); err != nil {
			return nil, err
		}
		if row.Level == 0 {
			continue
		}
		templates.oilByLevel[row.Level] = row
		if row.Level > templates.maxLevel {
			templates.maxLevel = row.Level
		}
	}

	if templates.maxLevel == 0 {
		templates.maxLevel = 1
		templates.oilByLevel[1] = oilfieldRuntimeTemplate{Level: 1, HourTime: 1}
	}

	return templates, nil
}

func normalizeNavalAcademyRuntime(runtime *orm.NavalAcademyRuntime, templates *academyRuntimeTemplates, nowUnix uint32, baseline uint32) bool {
	changed := false

	runtime.OilWellLevel, changed = clampAcademyLevel(runtime.OilWellLevel, templates.maxLevel, changed)
	runtime.GoldWellLevel, changed = clampAcademyLevel(runtime.GoldWellLevel, templates.maxLevel, changed)

	if runtime.OilCollectTimestamp == 0 && baseline > 0 {
		runtime.OilCollectTimestamp = baseline
		changed = true
	}
	if runtime.GoldCollectTimestamp == 0 && baseline > 0 {
		runtime.GoldCollectTimestamp = baseline
		changed = true
	}

	changed = finalizeUpgradeIfDone(&runtime.OilWellLevel, &runtime.OilUpgradeStartTime, &runtime.OilUpgradeCompleteTime, &runtime.OilCollectTimestamp, templates.maxLevel, nowUnix, changed)
	changed = finalizeUpgradeIfDone(&runtime.GoldWellLevel, &runtime.GoldUpgradeStartTime, &runtime.GoldUpgradeCompleteTime, &runtime.GoldCollectTimestamp, templates.maxLevel, nowUnix, changed)

	if runtime.OilUpgradeCompleteTime > nowUnix && runtime.OilUpgradeStartTime == 0 {
		runtime.OilUpgradeStartTime = inferUpgradeStart(runtime.OilWellLevel, runtime.OilUpgradeCompleteTime, templates)
		if runtime.OilUpgradeStartTime > 0 {
			changed = true
		}
	}
	if runtime.GoldUpgradeCompleteTime > nowUnix && runtime.GoldUpgradeStartTime == 0 {
		runtime.GoldUpgradeStartTime = inferUpgradeStart(runtime.GoldWellLevel, runtime.GoldUpgradeCompleteTime, templates)
		if runtime.GoldUpgradeStartTime > 0 {
			changed = true
		}
	}

	if runtime.OilUpgradeCompleteTime > 0 && runtime.OilUpgradeCompleteTime <= runtime.OilUpgradeStartTime {
		runtime.OilUpgradeStartTime = 0
		runtime.OilUpgradeCompleteTime = 0
		changed = true
	}
	if runtime.GoldUpgradeCompleteTime > 0 && runtime.GoldUpgradeCompleteTime <= runtime.GoldUpgradeStartTime {
		runtime.GoldUpgradeStartTime = 0
		runtime.GoldUpgradeCompleteTime = 0
		changed = true
	}

	return changed
}

func clampAcademyLevel(level uint32, maxLevel uint32, changed bool) (uint32, bool) {
	if level == 0 {
		return 1, true
	}
	if level > maxLevel {
		return maxLevel, true
	}
	return level, changed
}

func finalizeUpgradeIfDone(level *uint32, start *uint32, finish *uint32, collect *uint32, maxLevel uint32, nowUnix uint32, changed bool) bool {
	if *finish == 0 || *finish > nowUnix {
		return changed
	}
	if *level < maxLevel {
		*level = *level + 1
	}
	if *collect < *finish {
		*collect = *finish
	}
	*start = 0
	*finish = 0
	return true
}

func inferUpgradeStart(level uint32, finish uint32, templates *academyRuntimeTemplates) uint32 {
	template, ok := templates.oilByLevel[level]
	if !ok || template.Time == 0 || finish <= template.Time {
		return 0
	}
	return finish - template.Time
}

func computeFacilityAccrual(lastCollect uint32, nowUnix uint32, upgradeStart uint32, upgradeEnd uint32, template oilfieldRuntimeTemplate) uint32 {
	if lastCollect == 0 || lastCollect >= nowUnix {
		return 0
	}

	cycleSeconds := template.HourTime * 3600
	if cycleSeconds == 0 || template.Production == 0 || template.Store == 0 {
		return 0
	}

	activeSeconds := uint64(nowUnix - lastCollect)
	if upgradeEnd > lastCollect && upgradeStart < nowUnix && upgradeEnd > upgradeStart {
		overlapStart := upgradeStart
		if overlapStart < lastCollect {
			overlapStart = lastCollect
		}
		overlapEnd := upgradeEnd
		if overlapEnd > nowUnix {
			overlapEnd = nowUnix
		}
		if overlapEnd > overlapStart {
			activeSeconds -= uint64(overlapEnd - overlapStart)
		}
	}

	gain := uint32((activeSeconds * uint64(template.Production)) / uint64(cycleSeconds))
	if gain > template.Store {
		return template.Store
	}
	return gain
}
