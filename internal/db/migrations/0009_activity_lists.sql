-- data1_list becomes a JSON array (db.IDList) instead of comma separated.
UPDATE commander_activities SET data1_list = '[' || data1_list || ']';
