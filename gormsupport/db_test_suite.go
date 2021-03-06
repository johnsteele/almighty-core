package gormsupport

import (
	"os"

	"github.com/Sirupsen/logrus"
	config "github.com/almighty/almighty-core/configuration"
	"github.com/almighty/almighty-core/resource"

	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq" // need to import postgres driver
	"github.com/stretchr/testify/suite"
)

var _ suite.SetupAllSuite = &DBTestSuite{}
var _ suite.TearDownAllSuite = &DBTestSuite{}

func NewDBTestSuite(configFilePath string) DBTestSuite {
	return DBTestSuite{configFile: configFilePath}
}

// DBTestSuite is a base for tests using a gorm db
type DBTestSuite struct {
	suite.Suite
	configFile string
	DB         *gorm.DB
}

// SetupSuite implements suite.SetupAllSuite
func (s *DBTestSuite) SetupSuite() {
	resource.Require(s.T(), resource.Database)
	configuration, err := config.NewConfigurationData(s.configFile)
	if err != nil {
		logrus.Panic(nil, map[string]interface{}{
			"err": err,
		}, "failed to setup the configuration")
	}

	if _, c := os.LookupEnv(resource.Database); c != false {
		s.DB, err = gorm.Open("postgres", configuration.GetPostgresConfigString())
		if err != nil {
			panic("Failed to connect database: " + err.Error())
		}

	}

}

// TearDownSuite implements suite.TearDownAllSuite
func (s *DBTestSuite) TearDownSuite() {
	s.DB.Close()
}
