// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"database/sql"
	"github.com/lib/pq"
	"github.com/matrix-org/dendrite/roomserver/types"
)

const roomsSchema = `
CREATE SEQUENCE IF NOT EXISTS room_nid_seq;
CREATE TABLE IF NOT EXISTS rooms (
    -- Local numeric ID for the room.
    room_nid BIGINT PRIMARY KEY DEFAULT nextval('room_nid_seq'),
    -- Textual ID for the room.
    room_id TEXT NOT NULL CONSTRAINT room_id_unique UNIQUE,
    -- The most recent events in the room that aren't referenced by another event.
    -- This list may empty if the server hasn't joined the room yet.
    -- (The server will be in that state while it stores the events for the initial state of the room)
    latest_event_nids BIGINT[] NOT NULL DEFAULT '{}'::BIGINT[],
    -- The last event written to the output log for this room.
    last_event_sent_nid BIGINT NOT NULL DEFAULT 0,
    -- The state of the room after the current set of latest events.
    -- This will be 0 if there are no latest events in the room.
    state_snapshot_nid BIGINT NOT NULL DEFAULT 0
);
`

// Same as insertEventTypeNIDSQL
const insertRoomNIDSQL = "" +
	"INSERT INTO rooms (room_id) VALUES ($1)" +
	" ON CONFLICT ON CONSTRAINT room_id_unique" +
	" DO NOTHING RETURNING (room_nid)"

const selectRoomNIDSQL = "" +
	"SELECT room_nid FROM rooms WHERE room_id = $1"

const selectLatestEventNIDsSQL = "" +
	"SELECT latest_event_nids, state_snapshot_nid FROM rooms WHERE room_nid = $1"

const selectLatestEventNIDsForUpdateSQL = "" +
	"SELECT latest_event_nids, last_event_sent_nid, state_snapshot_nid FROM rooms WHERE room_nid = $1 FOR UPDATE"

const updateLatestEventNIDsSQL = "" +
	"UPDATE rooms SET latest_event_nids = $2, last_event_sent_nid = $3, state_snapshot_nid = $4 WHERE room_nid = $1"

type roomStatements struct {
	insertRoomNIDStmt                  *sql.Stmt
	selectRoomNIDStmt                  *sql.Stmt
	selectLatestEventNIDsStmt          *sql.Stmt
	selectLatestEventNIDsForUpdateStmt *sql.Stmt
	updateLatestEventNIDsStmt          *sql.Stmt
}

func (s *roomStatements) prepare(db *sql.DB) (err error) {
	_, err = db.Exec(roomsSchema)
	if err != nil {
		return
	}
	return statementList{
		{&s.insertRoomNIDStmt, insertRoomNIDSQL},
		{&s.selectRoomNIDStmt, selectRoomNIDSQL},
		{&s.selectLatestEventNIDsStmt, selectLatestEventNIDsSQL},
		{&s.selectLatestEventNIDsForUpdateStmt, selectLatestEventNIDsForUpdateSQL},
		{&s.updateLatestEventNIDsStmt, updateLatestEventNIDsSQL},
	}.prepare(db)
}

func (s *roomStatements) insertRoomNID(roomID string) (types.RoomNID, error) {
	var roomNID int64
	err := s.insertRoomNIDStmt.QueryRow(roomID).Scan(&roomNID)
	return types.RoomNID(roomNID), err
}

func (s *roomStatements) selectRoomNID(roomID string) (types.RoomNID, error) {
	var roomNID int64
	err := s.selectRoomNIDStmt.QueryRow(roomID).Scan(&roomNID)
	return types.RoomNID(roomNID), err
}

func (s *roomStatements) selectLatestEventNIDs(roomNID types.RoomNID) ([]types.EventNID, types.StateSnapshotNID, error) {
	var nids pq.Int64Array
	var stateSnapshotNID int64
	err := s.selectLatestEventNIDsStmt.QueryRow(int64(roomNID)).Scan(&nids, &stateSnapshotNID)
	if err != nil {
		return nil, 0, err
	}
	eventNIDs := make([]types.EventNID, len(nids))
	for i := range nids {
		eventNIDs[i] = types.EventNID(nids[i])
	}
	return eventNIDs, types.StateSnapshotNID(stateSnapshotNID), nil
}

func (s *roomStatements) selectLatestEventsNIDsForUpdate(txn *sql.Tx, roomNID types.RoomNID) (
	[]types.EventNID, types.EventNID, types.StateSnapshotNID, error,
) {
	var nids pq.Int64Array
	var lastEventSentNID int64
	var stateSnapshotNID int64
	err := txn.Stmt(s.selectLatestEventNIDsForUpdateStmt).QueryRow(int64(roomNID)).Scan(&nids, &lastEventSentNID, &stateSnapshotNID)
	if err != nil {
		return nil, 0, 0, err
	}
	eventNIDs := make([]types.EventNID, len(nids))
	for i := range nids {
		eventNIDs[i] = types.EventNID(nids[i])
	}
	return eventNIDs, types.EventNID(lastEventSentNID), types.StateSnapshotNID(stateSnapshotNID), nil
}

func (s *roomStatements) updateLatestEventNIDs(
	txn *sql.Tx, roomNID types.RoomNID, eventNIDs []types.EventNID, lastEventSentNID types.EventNID,
	stateSnapshotNID types.StateSnapshotNID,
) error {
	_, err := txn.Stmt(s.updateLatestEventNIDsStmt).Exec(
		roomNID, eventNIDsAsArray(eventNIDs), int64(lastEventSentNID), int64(stateSnapshotNID),
	)
	return err
}
