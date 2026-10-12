package testutil

import "testing"

func TestMediaSessionsMigrationRollback(t *testing.T) {
	db := Database(t)
	var owner, conversation, attachment int64
	if err := db.QueryRow(`INSERT INTO users(first_name,email,password_hash) VALUES('A','sessions@test.local','fixture') RETURNING id`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO conversations DEFAULT VALUES RETURNING id`).Scan(&conversation); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO conversation_members(conversation_id,user_id) VALUES($1,$2)`, conversation, owner); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO attachments(conversation_id,owner_id,object_key,declared_content_type,declared_size_bytes) VALUES($1,$2,'migration-fixture','image/jpeg',100) RETURNING id`, conversation, owner).Scan(&attachment); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE attachments SET cleanup_token='claim' WHERE id=$1`, attachment); err == nil {
		t.Fatal("partial cleanup claim accepted")
	}
	for _, column := range []string{"session_cipher=decode('00','hex')", "cleanup_token='claim',cleanup_claimed_at=now()"} {
		if _, err := db.Exec(`UPDATE attachments SET `+column+` WHERE id=$1`, attachment); err != nil {
			t.Fatal(err)
		}
		assertAttachmentSQLError(t, mediaMigration(t, db, "000005_media_sessions", "down"), "P0001")
		if _, err := db.Exec(`UPDATE attachments SET session_cipher=NULL,cleanup_token=NULL,cleanup_claimed_at=NULL WHERE id=$1`, attachment); err != nil {
			t.Fatal("refusal lost recovery columns")
		}
	}
	if err := mediaMigration(t, db, "000005_media_sessions", "down"); err != nil {
		t.Fatal(err)
	}
	if err := mediaMigration(t, db, "000005_media_sessions", "up"); err != nil {
		t.Fatal(err)
	}
	var size int64
	if err := db.QueryRow(`SELECT declared_size_bytes FROM attachments WHERE id=$1`, attachment).Scan(&size); err != nil || size != 100 {
		t.Fatal("session migration changed attachment metadata")
	}
}
