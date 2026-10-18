-- Requisitions (SupportShipCommand, CS_16100) made in a month: month is the
-- start of the month after it (when the count resets), count the ships
-- requisitioned, held against gameset supports_config description[3]
CREATE TABLE commander_requisitions (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	month        BIGINT  NOT NULL,
	count        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, month)
);
