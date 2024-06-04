package suggest_test

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/suggest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiagnose(t *testing.T) {
	res, err := suggest.Diagnose(context.Background(), "./testSpec.yaml")
	assert.NoError(t, err)
	assert.NotNil(t, res)

	assert.EqualValues(t, []string{"update_pet"}, res.InconsistentCasing)
	assert.EqualValues(t, []string{"addPet"}, res.MissingTags)
	assert.EqualValues(t, []string{"my_users"}, res.InconsistentTags)
	assert.EqualValues(t, []string{
		"update_pet",
		"findPetsByStatus",
		"findPetsByTags",
		"getPetById",
		"deletePet",
		"createUsersWithListInput",
		"loginUser",
		"logoutUser",
		"getUserByName",
		"updateUser",
		"deleteUser",
	}, res.DuplicateInformation)
}
