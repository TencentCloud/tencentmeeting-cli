package cmdutil

import "tmeet/internal/exception"

const (
	// InviteesListMax is the maximum number of invitees that can be invited at once
	InviteesListMax = 100
	// MeetingControlUsersListMax is the maximum number of meeting control users that can be invited at once
	MeetingControlUsersListMax = 20
)

// ApiInviteesUser invitees user
type ApiInviteesUser struct {
	Userid string `json:"userid"` // user id
}

// ApiMeetingControlUser meeting control user
type ApiMeetingControlUser struct {
	ToOperatorId     string `json:"to_operator_id"`      // to operator id
	ToOperatorIdType int    `json:"to_operator_id_type"` // to operator id type
	InstanceId       int    `json:"instanceid"`          // instance id
}

// PackageApiInviteesUsers package api invitees
func PackageApiInviteesUsers(flag string, openIdList []string) ([]*ApiInviteesUser, error) {
	inviteesMap := make(map[string]bool, len(openIdList))
	for i, openID := range openIdList {
		if openID == "" {
			return nil, exception.InvalidArgsError.With("flag: %s[%d].open_id not be empty", flag, i)
		}

		inviteesMap[openID] = true
	}

	if len(inviteesMap) > InviteesListMax {
		return nil, exception.InvalidArgsError.With("invitees list is too long, max is %d", InviteesListMax)
	}

	invitees := make([]*ApiInviteesUser, 0, len(inviteesMap))
	for openID := range inviteesMap {
		invitees = append(invitees, &ApiInviteesUser{
			Userid: openID,
		})
	}

	return invitees, nil
}

// PackageMeetingControlUsers package meeting control users
func PackageMeetingControlUsers(flag string, openIdList []string) ([]*ApiMeetingControlUser, error) {
	meetingControlUsersMap := make(map[string]bool, len(openIdList))
	for i, openID := range openIdList {
		if openID == "" {
			return nil, exception.InvalidArgsError.With("flag: %s[%d].open_id not be empty", flag, i)
		}

		meetingControlUsersMap[openID] = true
	}

	if len(meetingControlUsersMap) > MeetingControlUsersListMax {
		return nil, exception.InvalidArgsError.With(
			"meeting control users list is too long, max is %d", MeetingControlUsersListMax)
	}

	meetingControlUsers := make([]*ApiMeetingControlUser, 0, len(meetingControlUsersMap))
	for openID := range meetingControlUsersMap {
		meetingControlUsers = append(meetingControlUsers, &ApiMeetingControlUser{
			ToOperatorId:     openID,
			ToOperatorIdType: 2, // openId
		})
	}

	return meetingControlUsers, nil
}

// PackageMeetingControlSpecialUsers package meeting control special users
func PackageMeetingControlSpecialUsers(flag string, msOpenIdList []string, instanceId int) ([]*ApiMeetingControlUser, error) {
	meetingControlUsersMap := make(map[string]bool, len(msOpenIdList))
	for i, openID := range msOpenIdList {
		if openID == "" {
			return nil, exception.InvalidArgsError.With("flag: %s[%d].open_id not be empty", flag, i)
		}

		meetingControlUsersMap[openID] = true
	}

	if len(meetingControlUsersMap) > MeetingControlUsersListMax {
		return nil, exception.InvalidArgsError.With(
			"meeting control users list is too long, max is %d", MeetingControlUsersListMax)
	}

	meetingControlUsers := make([]*ApiMeetingControlUser, 0, len(meetingControlUsersMap))
	for openID := range meetingControlUsersMap {
		meetingControlUsers = append(meetingControlUsers, &ApiMeetingControlUser{
			ToOperatorId:     openID,
			ToOperatorIdType: 4, // ms_open_id
			InstanceId:       instanceId,
		})
	}

	return meetingControlUsers, nil
}
