package database

import (
	"errors"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"log"
)

type ContributorModel struct {
	gorm.Model

	Name           string
	Current_bounty int `gorm:"default:0"`
}

type MaintainerModel struct {
	Username string `gorm:"primaryKey"`
}

type ContributorRecordModel struct {
	gorm.Model

	Contributor_name string
	Maintainer_name  string
	Pullreq_url      string
	Points_allotted  int
}

// TODO Implement method to connect GORM based on connection
// String
// Return GORM instance to store on main struct

// Manager struct
type DBManager struct {
	db *gorm.DB
}

func (manager *DBManager) Init(connection_string string) error {

	log.Println("[DBMANAGER] Initializing Database")
	// Initialize The GORM DB interface
	db, err := gorm.Open(sqlite.Open(connection_string), &gorm.Config{})
	if err != nil {
		log.Println("[ERROR][DBMANAGER] Could not initialize Database ->", err)
		return err
	} else {
		manager.db = db
		log.Println("[DBMANAGER] Successfully Initialized Database")
	}

	log.Println("[DBMANAGER] Beginning Model Automigration")

	err = manager.db.AutoMigrate(&ContributorModel{})
	if err != nil {
		log.Println("[ERROR][DBMANAGER] Could not AutoMigrate ContributorModel ->", err)
		return err
	} else {
		log.Println("[DBMANAGER] Successfully AutoMigrated ContributorModel")
	}

	err = manager.db.AutoMigrate(&ContributorRecordModel{})
	if err != nil {
		log.Println("[ERROR][DBMANAGER] Could not AutoMigrate ContributorRecordModel ->", err)
		return err
	} else {
		log.Println("[DBMANAGER] Successfully AutoMigrated ContributorRecordModel")
	}

	err = manager.db.AutoMigrate(&MaintainerModel{})
	if err != nil {
		log.Println("[ERROR][DBMANAGER] Could not AutoMigrate MaintainerModel ->", err)
		return err
	} else {
		log.Println("[DBMANAGER] Successfully AutoMigrated MaintainerModel")
	}

	return nil
}

func (manager *DBManager) AssignBounty(
	maintainer string,
	contributor string,
	pr_html_url string,
	bounty_points int,
) error {

	// TODO Handle for Re-assignment
	// Start a New Transaction to create this object

	log.Println("[DBMANAGER][BOUNTY] Beginning Transaction to Assign Bounty")
	// Create the dummy record for the contributor_model
	// contributor_model := ContributorModel{name: contributor}

	// Create the time-series record of this transaction
	log.Println("[DBMANAGER][BOUNTY] Creating Contributor Record Model")

	crm := ContributorRecordModel{
		Maintainer_name:  maintainer,
		Contributor_name: contributor,
		Pullreq_url:      pr_html_url,
		Points_allotted:  bounty_points,
	}

	// Create the user struct
	// contributor_temp_representation := ContributorModel{
	// 	Name:           contributor,
	// 	Current_bounty: bounty_points,
	// }

	log.Println("[DBMANAGER][BOUNTY] Creating Contributor Record Model -> ", crm)
	log.Println("[DBMANAGER][BOUNTY] Beginning Transaction -> ", crm)

	manager.db.Transaction(func(tx *gorm.DB) error {

		// Create the time-series record
		result := tx.Create(&crm)
		if result.Error != nil {

			// Edge Case - User record already exists in time-series data
			// In that case, update that

			log.Println("[ERROR][DBMANAGER][BOUNTY] Could Not Create ContributorRecordModel ->", result.Error)
			return result.Error
		} else {
			log.Println("[DBMANAGER][BOUNTY] Successfully Created Contributor Record Model")
		}

		// default case - assume the user does not exist

		/*
			// Test if the user exists by attempting to create the user as
			// a new record
			user_create_result := tx.Create(&contributor_temp_representation)

			if user_create_result.Error != nil {
				// Check for the case where the user already exists

				// if that's the case, update the bounty with the new points

				// Else, report the error -> We found somethin unexpected

			} else {
				// Set the Bounty values
				// No Error, you can use this newly created user
				return nil
			}
		*/

		log.Println("[DBMANAGER][LEADERBOARD] Beginning Recompute of ContributorModel")

		//Recompute ContributorModel Table
		lb_query := `DELETE FROM contributor_models;INSERT INTO contributor_models (Name, Current_bounty)
SELECT contributor_name AS Name, sum(latest_points) AS Current_bounty from (
   select
       contributor_name, (SELECT points_allotted FROM contributor_record_models where t1.pullreq_url = pullreq_url order by created_at desc limit 1) as latest_points
   from contributor_record_models as t1
   GROUP by pullreq_url, contributor_name
) GROUP BY contributor_name;`

		result = tx.Exec(lb_query)
		if result.Error != nil {
			log.Println("[ERROR][DBMANAGER][LEADERBOARD] Could Not Recompute ContributorModel ->", result.Error)
			return result.Error
		} else {
			log.Println("[DBMANAGER][LEADERBOARD] Successfully Recomputed ContributorModel")
		}
		// commit the transaction
		return nil
	})

	return nil
}

func (manager *DBManager) Get_all_records() ([]ContributorRecordModel, error) {

	// Declare the array of all records
	var records []ContributorRecordModel

	// Fetch from the database
	log.Println("[DBMANAGER|RECORDS] Fetching All Records")
	fetch_result := manager.db.Find(&records)
	if fetch_result.Error != nil {
		log.Println("[ERROR][DBMANAGER|RECORDS] Could not fetch all records ->", fetch_result.Error)
		return nil, fetch_result.Error
	} else {
		log.Println("[DBMANAGER|RECORDS] Successfully Fetched all records")
		return records, nil
	}

}

func (manager *DBManager) Get_user_records(contributor string) ([]ContributorRecordModel, error) {
	query := `select * from contributor_record_models
         where contributor_name like ?
         order by created_at desc;`

	// Declare the array of all records
	var records []ContributorRecordModel

	// Fetch from the database
	log.Println("[DBMANAGER|USER-SPECIFIC] Fetching Records for user:", contributor)

	fetch_result := manager.db.Raw(query, contributor).Scan(&records)

	if fetch_result.Error != nil {
		log.Println("[ERROR][DBMANAGER|USER-SPECIFIC] Could not fetch records for", contributor, " ->", fetch_result.Error)
		return nil, fetch_result.Error
	} else {
		log.Println("[DBMANAGER|USER-SPECIFIC] Successfully Fetched all records for user:", contributor)
		return records, nil
	}
}

func (manager *DBManager) Get_leaderboard() ([]ContributorModel, error) {

	leaderboard_query := `
	SELECT contributor_name AS Name, sum(latest_points) AS Current_bounty from (
		select
			contributor_name, (SELECT points_allotted FROM contributor_record_models where t1.pullreq_url = pullreq_url order by created_at desc limit 1) as latest_points
		from contributor_record_models as t1
		GROUP by pullreq_url, contributor_name
	) GROUP BY contributor_name;
	`

	// Declare the array of all records
	var records []ContributorModel

	// Fetch from the database
	log.Println("[DBMANAGER|LEADERBOARD] Fetching All Records")

	fetch_result := manager.db.Raw(leaderboard_query).Scan(&records)

	if fetch_result.Error != nil {
		log.Println("[ERROR][DBMANAGER|LEADERBOARD] Could not fetch all records ->", fetch_result.Error)
		return nil, fetch_result.Error
	} else {
		log.Println("[DBMANAGER|LEADERBOARD] Successfully Fetched all records")
		return records, nil
	}

}

func (manager *DBManager) Get_leaderboard_mat() ([]ContributorModel, error) {
	// Declare the array of all records
	var records []ContributorModel

	// Fetch from the database
	//log.Println("[DBMANAGER|MUX-LB] Fetching All Records")
	fetch_result := manager.db.Find(&records)
	if fetch_result.Error != nil {
		log.Println("[ERROR][DBMANAGER|MUX-LB] Could not fetch all records ->", fetch_result.Error)
		return nil, fetch_result.Error
	} else {
		//log.Println("[DBMANAGER|MUX-LB] Successfully Fetched all records")
		return records, nil
	}
}

func (manager *DBManager) Check_is_maintainer(user_name string) (bool, error) {
	var maintainer MaintainerModel

	log.Printf("[DBMANAGER|CHECK] Checking if %s is a maintainer\n", user_name)
	result := manager.db.Limit(1).First(&maintainer, "username like ?", user_name)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			log.Printf("[DBMANAGER|CHECK] %s IS NOT a maintainer\n", user_name)
			return false, nil
		}
		log.Println("[ERROR][DBMANAGER|CHECK] Could not check maintainer ->", result.Error)
		return false, result.Error
	}

	log.Printf("[DBMANAGER|CHECK] %s IS a maintainer\n", user_name)
	return true, nil
}
