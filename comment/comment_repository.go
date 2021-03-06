package comment

import (
	"context"
	"time"

	"github.com/almighty/almighty-core/errors"
	"github.com/almighty/almighty-core/log"
	"github.com/almighty/almighty-core/rendering"
	"github.com/goadesign/goa"
	"github.com/jinzhu/gorm"

	errs "github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

// Repository describes interactions with comments
type Repository interface {
	Create(ctx context.Context, comment *Comment, creator uuid.UUID) error
	Save(ctx context.Context, comment *Comment, modifier uuid.UUID) error
	Delete(ctx context.Context, commentID uuid.UUID, suppressor uuid.UUID) error
	List(ctx context.Context, parent string, start *int, limit *int) ([]*Comment, uint64, error)
	Load(ctx context.Context, id uuid.UUID) (*Comment, error)
	Count(ctx context.Context, parent string) (int, error)
}

// NewRepository creates a new storage type.
func NewRepository(db *gorm.DB) Repository {
	return &GormCommentRepository{db: db, revisionRepository: &GormCommentRevisionRepository{db}}
}

// GormCommentRepository is the implementation of the storage interface for Comments.
type GormCommentRepository struct {
	db                 *gorm.DB
	revisionRepository RevisionRepository
}

// TableName overrides the table name settings in Gorm to force a specific table name
// in the database.
func (m *GormCommentRepository) TableName() string {
	return "comments"
}

// Create creates a new record.
func (m *GormCommentRepository) Create(ctx context.Context, comment *Comment, creatorID uuid.UUID) error {
	defer goa.MeasureSince([]string{"goa", "db", "comment", "create"}, time.Now())
	comment.ID = uuid.NewV4()
	// make sure no comment is created with an empty 'markup' value
	if comment.Markup == "" {
		comment.Markup = rendering.SystemMarkupDefault
	}
	if err := m.db.Create(comment).Error; err != nil {
		log.Error(ctx, map[string]interface{}{
			"commentID": comment.ID,
			"err":       err,
		}, "unable to create the comment")
		return errs.WithStack(err)
	}
	if err := m.revisionRepository.Create(ctx, creatorID, RevisionTypeCreate, *comment); err != nil {
		return errs.WithStack(err)
	}
	log.Debug(ctx, map[string]interface{}{
		"commentID": comment.ID,
	}, "Comment created!")

	return nil
}

// Save a single comment
func (m *GormCommentRepository) Save(ctx context.Context, comment *Comment, modifierID uuid.UUID) error {
	c := Comment{}
	tx := m.db.Where("id=?", comment.ID).First(&c)
	if tx.RecordNotFound() {
		log.Error(ctx, map[string]interface{}{
			"commentID": comment.ID,
		}, "comment not found!")
		// treating this as a not found error: the fact that we're using number internal is implementation detail
		return errors.NewNotFoundError("comment", comment.ID.String())
	}
	if err := tx.Error; err != nil {
		log.Error(ctx, map[string]interface{}{
			"commentID": comment.ID,
			"err":       err,
		}, "comment search operation failed!")

		return errors.NewInternalError(err.Error())
	}
	// make sure no comment is created with an empty 'markup' value
	if comment.Markup == "" {
		comment.Markup = rendering.SystemMarkupDefault
	}
	tx = tx.Save(comment)
	if err := tx.Error; err != nil {
		log.Error(ctx, map[string]interface{}{
			"commentID": comment.ID,
			"err":       err,
		}, "unable to save the comment!")

		return errors.NewInternalError(err.Error())
	}
	if err := m.revisionRepository.Create(ctx, modifierID, RevisionTypeUpdate, *comment); err != nil {
		return errs.WithStack(err)
	}
	log.Debug(ctx, map[string]interface{}{
		"commentID": comment.ID,
	}, "Comment updated!")

	return nil
}

// Delete a single comment
func (m *GormCommentRepository) Delete(ctx context.Context, commentID uuid.UUID, suppressorID uuid.UUID) error {
	if commentID == uuid.Nil {
		return errors.NewNotFoundError("comment", commentID.String())
	}
	// fetch the id and parent id of the comment to delete, to store them in the new revision.
	c := Comment{}
	tx := m.db.Select("id, parent_id").Where("id = ?", commentID).Find(&c)
	m.db.Delete(c)
	if tx.RowsAffected == 0 {
		return errors.NewNotFoundError("comment", commentID.String())
	}
	if err := tx.Error; err != nil {
		return errors.NewInternalError(err.Error())
	}
	if err := m.revisionRepository.Create(ctx, suppressorID, RevisionTypeDelete, c); err != nil {
		return errs.WithStack(err)
	}
	return nil
}

// List all comments related to a single item
func (m *GormCommentRepository) List(ctx context.Context, parent string, start *int, limit *int) ([]*Comment, uint64, error) {
	defer goa.MeasureSince([]string{"goa", "db", "comment", "query"}, time.Now())

	db := m.db.Model(&Comment{}).Where("parent_id = ?", parent)
	orgDB := db
	if start != nil {
		if *start < 0 {
			return nil, 0, errors.NewBadParameterError("start", *start)
		}
		db = db.Offset(*start)
	}
	if limit != nil {
		if *limit <= 0 {
			return nil, 0, errors.NewBadParameterError("limit", *limit)
		}
		db = db.Limit(*limit)
	}
	db = db.Select("count(*) over () as cnt2 , *").Order("created_at desc")

	rows, err := db.Rows()
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []*Comment{}
	columns, err := rows.Columns()
	if err != nil {
		return nil, 0, errors.NewInternalError(err.Error())
	}

	// need to set up a result for Scan() in order to extract total count.
	var count uint64
	var ignore interface{}
	columnValues := make([]interface{}, len(columns))

	for index := range columnValues {
		columnValues[index] = &ignore
	}
	columnValues[0] = &count
	first := true

	for rows.Next() {
		value := &Comment{}
		db.ScanRows(rows, value)
		if first {
			first = false
			if err = rows.Scan(columnValues...); err != nil {
				return nil, 0, errors.NewInternalError(err.Error())
			}
		}
		result = append(result, value)

	}
	if first {
		// means 0 rows were returned from the first query (maybe because of offset outside of total count),
		// need to do a count(*) to find out total
		orgDB := orgDB.Select("count(*)")
		rows2, err := orgDB.Rows()
		defer rows2.Close()
		if err != nil {
			return nil, 0, err
		}
		rows2.Next() // count(*) will always return a row
		rows2.Scan(&count)
	}
	return result, count, nil
}

// Count all comments related to a single item
func (m *GormCommentRepository) Count(ctx context.Context, parent string) (int, error) {
	defer goa.MeasureSince([]string{"goa", "db", "comment", "query"}, time.Now())
	var count int

	m.db.Model(&Comment{}).Where("parent_id = ?", parent).Count(&count)

	return count, nil
}

// Load a single comment regardless of parent
func (m *GormCommentRepository) Load(ctx context.Context, id uuid.UUID) (*Comment, error) {
	defer goa.MeasureSince([]string{"goa", "db", "comment", "get"}, time.Now())
	var obj Comment

	tx := m.db.Where("id=?", id).First(&obj)
	if tx.RecordNotFound() {
		log.Error(ctx, map[string]interface{}{
			"commentID": id.String(),
		}, "comment search operation failed!")

		return nil, errors.NewNotFoundError("comment", id.String())
	}
	if tx.Error != nil {
		log.Error(ctx, map[string]interface{}{
			"commentID": id.String(),
			"err":       tx.Error,
		}, "unable to load the comment")

		return nil, errors.NewInternalError(tx.Error.Error())
	}
	return &obj, nil
}
