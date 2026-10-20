-- Akashi's affection in the shop (Player.chargeExp, SC_11003.acc_pay_lv):
-- exp, raised by ClickMingShiCommand (CS_11506, 5 a tap); taps the taps
-- counted on the server day of tapped_at (unix seconds, 0 never)
CREATE TABLE charge_exp (
	commander_id INTEGER NOT NULL PRIMARY KEY REFERENCES commanders (id),
	exp          INTEGER NOT NULL DEFAULT 0,
	taps         INTEGER NOT NULL DEFAULT 0,
	tapped_at    BIGINT  NOT NULL DEFAULT 0
);

-- redeem codes (server/exchange_codes.json) a commander used, once each
-- (CS_11508), at redeemed_at (unix seconds)
CREATE TABLE exchange_code_redemptions (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	code         TEXT    NOT NULL,
	redeemed_at  BIGINT  NOT NULL,
	PRIMARY KEY (commander_id, code)
);

-- questionnaires (survey_data_template) a commander took (CS_11025), at
-- done_at (unix seconds): SurveyStateCommand reports them done
CREATE TABLE surveys_done (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	survey_id    INTEGER NOT NULL,
	done_at      BIGINT  NOT NULL,
	PRIMARY KEY (commander_id, survey_id)
);
