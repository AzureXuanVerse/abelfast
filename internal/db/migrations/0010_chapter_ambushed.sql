-- ambushed becomes a JSON array of {row, column} cells (db.Cells) instead of
-- "row,column;...".
UPDATE chapter_states SET ambushed = CASE ambushed
	WHEN '' THEN '[]'
	ELSE '[[' || replace(ambushed, ';', '],[') || ']]'
END;
