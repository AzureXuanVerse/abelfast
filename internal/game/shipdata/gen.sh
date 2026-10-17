#!/usr/bin/env bash
# Regenerates the game data subsets embedded by internal/game/shipdata.go from
# the client's config tables converted to JSON (the al-data skill's table.sh
# output, e.g. .cache/al-data/<commit>/EN):
#
#   table.sh ship_data_template,ship_data_statistics,ship_data_group,ship_data_create_material,equip_data_template,compose_data_template,item_data_statistics EN
#   internal/game/shipdata/gen.sh .cache/al-data/<commit>/EN
set -euo pipefail

src=${1:?usage: gen.sh <converted region dir>}
out=$(dirname "$0")
table() {
	for dir in sharecfgdata ShareCfg; do
		if [ -s "$src/$dir/$1.json" ] && [ "$(jq 'length' "$src/$dir/$1.json")" != 0 ]; then
			echo "$src/$dir/$1.json"
			return
		fi
	done
	echo "missing table $1" >&2
	exit 1
}

# ships as obtained (base templates, id ending in 1): ship_data_template
# [star, energy, max_level, rarity, nationality, [equip_id_1..3], buff_list],
# rarity and nationality from ship_data_statistics
jq -c --slurpfile stats "$(table ship_data_statistics)" '
	$stats[0] as $s
	| [to_entries[] | select(.value | type == "object") | select(.key | endswith("1")) | select($s[.key] != null)
	| {key, value: [.value.star, .value.energy, .value.max_level, $s[.key].rarity, $s[.key].nationality,
		[.value.equip_id_1, .value.equip_id_2, .value.equip_id_3], .value.buff_list]}]
	| sort_by(.key | tonumber) | from_entries' "$(table ship_data_template)" \
	| sed 's/\]\],"/]],\n"/g' >"$out/ships.json"

# construction pools: ship_data_create_material costs and the rarity rates of
# its rate_tip, candidates from the ship_data_group descriptions that link to
# the pool (GETBOAT projectName)
jq -c --slurpfile groups "$(table ship_data_group)" '
	{"1": "special", "2": "light", "3": "heavy"} as $names
	| [$groups[0][] | select(type == "object")] as $g
	| [to_entries[] | select(.value | type == "object") | select($names[.key] != null) | .value
	| (.id | tostring) as $id
	| {key: $id, value: {
		gold: .use_gold, item: .use_item, item_count: .number_1, exchange_count: .exchange_count,
		rates: ([.rate_tip[] | gsub("<[^>]*>"; "") | capture("^(?<name>[A-Za-z ]+): (?<rate>[0-9.]+)%") | {(.name): (.rate | tonumber)}] | add),
		groups: [$g[] | select(any(.description[]?; .[1][0]? == "GETBOAT" and (.[1][1] | type) == "object" and .[1][1].projectName? == $names[$id])) | .group_type]
	}}] | from_entries' "$(table ship_data_create_material)" >"$out/pools.json"

# equipment upgrades: equip_data_template [next, trans_use_gold, trans_use_item]
jq -c '[to_entries[] | select(.value | type == "object") | select((.value.next // 0) != 0)
	| {key, value: [.value.next, .value.trans_use_gold, .value.trans_use_item]}] | sort_by(.key | tonumber) | from_entries' \
	"$(table equip_data_template)" | sed 's/\]\],"/]],\n"/g' >"$out/equip_upgrades.json"

# equipment crafting: compose_data_template [equip_id, gold_num, material_id, material_num]
jq -c '[to_entries[] | select(.value | type == "object")
	| {key, value: [.value.equip_id, .value.gold_num, .value.material_id, .value.material_num]}] | sort_by(.key | tonumber) | from_entries' \
	"$(table compose_data_template)" | sed 's/\],"/],\n"/g' >"$out/compose.json"

# item sell prices: item_data_statistics price [resource, amount]
jq -c '[to_entries[] | select(.value | type == "object") | select((.value.price // []) | length > 0)
	| {key, value: .value.price}] | sort_by(.key | tonumber) | from_entries' \
	"$(table item_data_statistics)" | sed 's/\],"/],\n"/g' >"$out/item_prices.json"
