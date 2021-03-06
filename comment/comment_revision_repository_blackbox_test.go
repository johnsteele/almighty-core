package comment_test

import (
	"context"
	"os"
	"testing"

	"github.com/almighty/almighty-core/account"
	"github.com/almighty/almighty-core/comment"
	"github.com/almighty/almighty-core/gormsupport"
	"github.com/almighty/almighty-core/gormsupport/cleaner"
	"github.com/almighty/almighty-core/migration"
	"github.com/almighty/almighty-core/models"
	"github.com/almighty/almighty-core/rendering"
	"github.com/almighty/almighty-core/resource"
	testsupport "github.com/almighty/almighty-core/test"
	"github.com/almighty/almighty-core/workitem"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestRunCommentRevisionRepositoryBlackBoxTest(t *testing.T) {
	resource.Require(t, resource.Database)
	suite.Run(t, &revisionRepositoryBlackBoxTest{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

type revisionRepositoryBlackBoxTest struct {
	gormsupport.DBTestSuite
	repository         comment.Repository
	revisionRepository comment.RevisionRepository
	clean              func()
	testIdentity1      account.Identity
	testIdentity2      account.Identity
	testIdentity3      account.Identity
}

// SetupSuite overrides the DBTestSuite's function but calls it before doing anything else
// The SetupSuite method will run before the tests in the suite are run.
// It sets up a database connection for all the tests in this suite without polluting global space.
func (s *revisionRepositoryBlackBoxTest) SetupSuite() {
	s.DBTestSuite.SetupSuite()
	// Make sure the database is populated with the correct types (e.g. bug etc.)
	if _, c := os.LookupEnv(resource.Database); c != false {
		if err := models.Transactional(s.DB, func(tx *gorm.DB) error {
			return migration.PopulateCommonTypes(context.Background(), tx, workitem.NewWorkItemTypeRepository(tx))
		}); err != nil {
			panic(err.Error())
		}
	}
	// s.DB.LogMode(true)
}

func (s *revisionRepositoryBlackBoxTest) SetupTest() {
	s.repository = comment.NewRepository(s.DB)
	s.revisionRepository = comment.NewRevisionRepository(s.DB)
	s.clean = cleaner.DeleteCreatedEntities(s.DB)
	testIdentity1, err := testsupport.CreateTestIdentity(s.DB, "jdoe1", "test")
	require.Nil(s.T(), err)
	s.testIdentity1 = testIdentity1
	testIdentity2, err := testsupport.CreateTestIdentity(s.DB, "jdoe2", "test")
	require.Nil(s.T(), err)
	s.testIdentity2 = testIdentity2
	testIdentity3, err := testsupport.CreateTestIdentity(s.DB, "jdoe3", "test")
	require.Nil(s.T(), err)
	s.testIdentity3 = testIdentity3
}

func (s *revisionRepositoryBlackBoxTest) TearDownTest() {
	s.clean()
}

func (s *revisionRepositoryBlackBoxTest) TestStoreCommentRevisions() {
	// given
	// create a comment
	c := newComment("A", "Body", rendering.SystemMarkupMarkdown)
	err := s.repository.Create(context.Background(), c, s.testIdentity1.ID)
	require.Nil(s.T(), err)
	// modify the comment
	c.Body = "Updated body"
	c.Markup = rendering.SystemMarkupPlainText
	err = s.repository.Save(
		context.Background(), c, s.testIdentity2.ID)
	require.Nil(s.T(), err)
	// modify again the comment
	c.Body = "Updated body2"
	c.Markup = rendering.SystemMarkupMarkdown
	err = s.repository.Save(context.Background(), c, s.testIdentity2.ID)
	require.Nil(s.T(), err)
	// delete the comment
	err = s.repository.Delete(
		context.Background(), c.ID, s.testIdentity3.ID)
	require.Nil(s.T(), err)
	// when
	commentRevisions, err := s.revisionRepository.List(context.Background(), c.ID)
	// then
	require.Nil(s.T(), err)
	require.Len(s.T(), commentRevisions, 4)
	// revision 1
	revision1 := commentRevisions[0]
	assert.Equal(s.T(), c.ID, revision1.CommentID)
	assert.Equal(s.T(), c.ParentID, revision1.CommentParentID)
	assert.Equal(s.T(), comment.RevisionTypeCreate, revision1.Type)
	assert.Equal(s.T(), "Body", *revision1.CommentBody)
	assert.Equal(s.T(), rendering.SystemMarkupMarkdown, *revision1.CommentMarkup)
	assert.Equal(s.T(), s.testIdentity1.ID, revision1.ModifierIdentity)
	// revision 2
	revision2 := commentRevisions[1]
	assert.Equal(s.T(), c.ID, revision2.CommentID)
	assert.Equal(s.T(), c.ParentID, revision2.CommentParentID)
	assert.Equal(s.T(), comment.RevisionTypeUpdate, revision2.Type)
	assert.Equal(s.T(), "Updated body", *revision2.CommentBody)
	assert.Equal(s.T(), rendering.SystemMarkupPlainText, *revision2.CommentMarkup)
	assert.Equal(s.T(), s.testIdentity2.ID, revision2.ModifierIdentity)
	// revision 3
	revision3 := commentRevisions[2]
	assert.Equal(s.T(), c.ID, revision3.CommentID)
	assert.Equal(s.T(), c.ParentID, revision3.CommentParentID)
	assert.Equal(s.T(), comment.RevisionTypeUpdate, revision3.Type)
	assert.Equal(s.T(), "Updated body2", *revision3.CommentBody)
	assert.Equal(s.T(), rendering.SystemMarkupMarkdown, *revision3.CommentMarkup)
	assert.Equal(s.T(), s.testIdentity2.ID, revision3.ModifierIdentity)
	// revision 4
	revision4 := commentRevisions[3]
	assert.Equal(s.T(), c.ID, revision4.CommentID)
	assert.Equal(s.T(), c.ParentID, revision4.CommentParentID)
	assert.Equal(s.T(), comment.RevisionTypeDelete, revision4.Type)
	assert.Nil(s.T(), revision4.CommentBody)
	assert.Nil(s.T(), revision4.CommentMarkup)
	assert.Equal(s.T(), s.testIdentity3.ID, revision4.ModifierIdentity)
}
