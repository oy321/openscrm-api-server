package models

import (
	"openscrm/app/requests"
	app "openscrm/common/app"
	"openscrm/common/log"
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CustomerExportItem struct {
	ExtID            string          `gorm:"type:char(64);uniqueIndex:idx_ext_customer_id;comment:微信定义的userID" json:"ext_customer_id"`
	CustomerName     string          `json:"customer_name"`
	CustomerCorpName string          `json:"customer_corp_name"`
	StaffName        string          `json:"staff_name"`
	Remark           string          `json:"remark"`
	Description      string          `json:"description"`
	Status           string          `json:"status"`
	Createtime       time.Time       `json:"createtime"`
	AddWay           int64           `json:"add_way"`
	Age              int64           `json:"age"`
	Gender           int64           `json:"gender"`
	Birthday         string          `json:"birthday"`
	PhoneNumber      string          `json:"phone_number"`
	Staffs           []CustomerStaff `gorm:"foreignKey:ExtCustomerID;references:ExtID" json:"staff_relations"`
}

type Customer struct {
	ExtCorpModel
	// 微信定义的客户ID
	ExtID string `gorm:"type:char(64);uniqueIndex:idx_ext_customer_id;comment:微信定义的userID" json:"ext_customer_id"`
	// 微信用户对应微信昵称；企业微信用户，则为联系人或管理员设置的昵称、认证的实名和账号名称
	Name string `gorm:"type:varchar(255);comment:名称，微信用户对应微信昵称；企业微信用户，则为联系人或管理员设置的昵称、认证的实名和账号名称" json:"name"`
	// 职位,客户为企业微信时使用
	Position string `gorm:"varchar(255);comment:职位,客户为企业微信时使用" json:"position"`
	// 客户的公司名称,仅当客户ID为企业微信ID时存在
	CorpName string `gorm:"type:varchar(255);comment:客户的公司名称,仅当客户ID为企业微信ID时存在" json:"corp_name"`
	// 头像
	Avatar string `gorm:"type:varchar(255);comment:头像" json:"avatar"`
	// 客户类型 1-微信用户, 2-企业微信用户
	Type int `gorm:"type:tinyint(1);index;comment:类型,1-微信用户, 2-企业微信用户" json:"type"`
	// 0-未知 1-男性 2-女性
	Gender  int    `gorm:"type:tinyint;comment:性别,0-未知 1-男性 2-女性" json:"gender"`
	Unionid string `gorm:"type:varchar(128);comment:微信开放平台的唯一身份标识(微信unionID)" json:"unionid"`
	// 仅当联系人类型是企业微信用户时有此字段
	ExternalProfile ExternalProfile `gorm:"type:json;comment:仅当联系人类型是企业微信用户时有此字段" json:"external_profile"`
	// 所属员工
	Staffs []CustomerStaff `gorm:"foreignKey:ExtCustomerID;references:ExtID" json:"staff_relations"`
	// 所属员工
	Timestamp
}

func (o *Customer) BeforeCreate(tx *gorm.DB) (err error) {
	if o.Avatar == "" {
		o.Avatar = "https://openscrm.oss-cn-hangzhou.aliyuncs.com/public/avatar.svg"
	}

	if o.Name == "" {
		o.Name = "未知"
	}

	return

}

func (o Customer) BatchUpsert(customers []Customer) (err error) {
	err = DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "ext_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"position", "name", "corp_name", "avatar", "type", "gender", "unionid", "external_profile"}),
	}).Omit("Staffs").CreateInBatches(&customers, 100).Error
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	return
}

func (o Customer) Upsert(customer Customer) error {
	updateFields := map[string]interface{}{
		"name":             customer.Name,
		"position":         customer.Position,
		"corp_name":        customer.CorpName,
		"avatar":           customer.Avatar,
		"type":             customer.Type,
		"gender":           customer.Gender,
		"unionid":          customer.Unionid,
		"external_profile": customer.ExternalProfile,
	}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "ext_id"}},
		DoUpdates: clause.Assignments(updateFields),
	}).Omit("Staffs").Create(&customer).Error
}

func (o Customer) Get(ID string, extCorpID string, withStaffRelation bool) (*Customer, error) {
	customer := Customer{}
	db := DB.Model(&Customer{}).Where("id = ? and ext_corp_id = ?", ID, extCorpID)
	if withStaffRelation {
		db = db.Preload("Staffs").Preload("Staffs.CustomerStaffTags")
	}
	err := db.Find(&customer).Error
	if err != nil {
		err = errors.Wrap(err, "Get customer by id failed")
		return &customer, err
	}
	return &customer, nil
}

//select c.name,
//       c.corp_name,
//       s.name,
//       customer_staff.remark,
//       customer_staff.description,
//       customer_staff.customer_delete_staff_at,
//       customer_staff.createtime,
//       customer_staff.add_way,
//       ci.age,
//       ci.birthday,
//       ci.phone_number
//from customer_staff
//         join customer c on c.ext_customer_id = customer_staff.ext_customer_id
//         join staff s on s.ext_staff_id = customer_staff.ext_staff_id
//         left join customer_info ci on c.id = ci.ext_customer_id;

func (o Customer) QueryExport(req requests.QueryCustomerReq, extCorpID string, pager *app.Pager) ([]CustomerExportItem, int64, error) {

	db := DB.Table("customer_staff").
		Joins("join customer c on c.ext_id = customer_staff.ext_customer_id").
		Joins("join staff s on s.ext_id = customer_staff.ext_staff_id").
		Joins("left join customer_info ci on c.id = ci.ext_customer_id")

	if req.Name != "" {
		db = db.Where("customer.name like ?", req.Name+"%")
	}
	if req.Gender != 0 {
		db = db.Where("customer.gender = ?", req.Gender)
	}
	if req.Type != 0 {
		db = db.Where("customer.type = ?", req.Type)
	}
	if len(req.ExtStaffIDs) > 0 {
		db = db.Where("cs.ext_staff_ids in (?)", req.ExtStaffIDs)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := app.GetPageOffset(pager.Page, pager.PageSize)
	if pageOffset >= 0 && pager.PageSize > 0 {
		db = db.Offset(pageOffset).Limit(pager.PageSize)
	}
	var res []CustomerExportItem
	if err := db.Preload("Staffs"). /*Preload("Staffs.CustomerStaffTags").*/ Select("customer.*").Find(&res).Error; err != nil {
		return nil, 0, err
	}
	return res, total, nil
}

// ExportQuery
// Description: 查询需要导出的客户
func (o Customer) ExportQuery(
	req requests.QueryCustomerReq, extCorpID string, pager *app.Pager) ([]*CustomerExportItem, int64, error) {

	var customers []*CustomerExportItem

	db := DB.Table("customer").
		Joins("left join customer_staff cs on customer.ext_id = cs.ext_customer_id").
		Joins("left join customer_staff_tag cst on cst.customer_staff_id = cs.id").
		Joins("join staff s on s.ext_id = cs.ext_staff_id").
		Joins("left join customer_info ci on customer.ext_id = ci.ext_customer_id").
		Where("cs.ext_corp_id = ?", extCorpID)

	if req.Name != "" {
		db = db.Where("customer.name like ?", req.Name+"%")
	}
	if req.Gender != 0 {
		db = db.Where("customer.gender = ?", req.Gender)
	}
	if req.Type != 0 {
		db = db.Where("customer.type = ?", req.Type)
	}
	if len(req.ExtStaffIDs) > 0 {
		db = db.Where("cs.ext_staff_id in (?)", req.ExtStaffIDs)
	}
	if req.StartTime != "" {
		db = db.Where("createtime between ? and ?", req.StartTime, req.EndTime)
	}
	if len(req.ExtTagIDs) > 0 {
		db = db.Where("cst.ext_tag_id in (?)", req.ExtTagIDs)
	}
	if req.ChannelType > 0 {
		db = db.Where("cs.add_way = ?", req.ChannelType)
	}

	var total int64
	if err := db.Distinct("customer.id").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	pager.SetDefault()
	db = db.Offset(pager.GetOffset()).Limit(pager.GetLimit())

	err := db.Preload("Staffs").
		Preload("Staffs.CustomerStaffTags").
		Select("customer.name as customer_name, customer.corp_name as customer_corp_name, " +
			" s.name as staff_name, cs.remark, cs.description, if(cs.deleted_at is null, '未流失', '已流失') as status," +
			" cs.createtime, cs.add_way, ci.age,  customer.gender, ci.birthday ,ci.phone_number").
		Find(&customers).Error
	if err != nil {
		err = errors.WithStack(err)
		return nil, 0, err
	}
	return customers, total, nil
}

// Query
// Description: 查询客户
func (o Customer) Query(
	req requests.QueryCustomerReq, extCorpID string, pager *app.Pager) ([]*Customer, int64, error) {

	var customers []*Customer

	// First try to get customers with staff relationships
	db := DB.Table("customer").
		Joins("left join customer_staff cs on customer.ext_id = cs.ext_customer_id").
		Joins("left join customer_staff_tag cst on cst.customer_staff_id = cs.id").
		Where("customer.ext_corp_id = ? OR cs.ext_corp_id = ?", extCorpID, extCorpID)

	if req.Name != "" {
		db = db.Where("customer.name like ?", req.Name+"%")
	}
	if req.Gender != 0 {
		db = db.Where("customer.gender = ?", req.Gender)
	}
	if req.Type != 0 {
		db = db.Where("customer.type = ?", req.Type)
	}
	if len(req.ExtStaffIDs) > 0 {
		db = db.Where("cs.ext_staff_id in (?)", req.ExtStaffIDs)
	}
	if req.StartTime != "" {
		db = db.Where("cs.createtime between ? and ?", req.StartTime, req.EndTime)
	}
	if len(req.ExtTagIDs) > 0 {
		db = db.Where("cst.ext_tag_id in (?)", req.ExtTagIDs)
	}
	if req.ChannelType > 0 {
		db = db.Where("cs.add_way = ?", req.ChannelType)
	}
	if req.OutFlowStatus == 1 {
		db = db.Unscoped().Where("cs.deleted_at is not null")
	} else if req.OutFlowStatus == 2 {
		db = db.Where("cs.deleted_at is null")
	}

	var total int64
	countErr := db.Distinct("customer.id").Count(&total).Error
	if countErr != nil {
		log.Sugar.Errorw("Customer count query failed", "error", countErr, "extCorpID", extCorpID)
		return nil, 0, countErr
	}

	log.Sugar.Infow("Customer query count", "total", total, "extCorpID", extCorpID)

	pager.SetDefault()

	// Create a new query for actual data retrieval to avoid GROUP BY issues
	// Get distinct customer IDs first, then fetch full customer records
	var customerIDs []string
	idQuery := DB.Table("customer").
		Joins("left join customer_staff cs on customer.ext_id = cs.ext_customer_id").
		Joins("left join customer_staff_tag cst on cst.customer_staff_id = cs.id").
		Where("customer.ext_corp_id = ? OR cs.ext_corp_id = ?", extCorpID, extCorpID)

	// Apply the same filters to ID query
	if req.Name != "" {
		idQuery = idQuery.Where("customer.name like ?", req.Name+"%")
	}
	if req.Gender != 0 {
		idQuery = idQuery.Where("customer.gender = ?", req.Gender)
	}
	if req.Type != 0 {
		idQuery = idQuery.Where("customer.type = ?", req.Type)
	}
	if len(req.ExtStaffIDs) > 0 {
		idQuery = idQuery.Where("cs.ext_staff_id in (?)", req.ExtStaffIDs)
	}
	if req.StartTime != "" {
		idQuery = idQuery.Where("cs.createtime between ? and ?", req.StartTime, req.EndTime)
	}
	if len(req.ExtTagIDs) > 0 {
		idQuery = idQuery.Where("cst.ext_tag_id in (?)", req.ExtTagIDs)
	}
	if req.ChannelType > 0 {
		idQuery = idQuery.Where("cs.add_way = ?", req.ChannelType)
	}
	if req.OutFlowStatus == 1 {
		idQuery = idQuery.Unscoped().Where("cs.deleted_at is not null")
	} else if req.OutFlowStatus == 2 {
		idQuery = idQuery.Where("cs.deleted_at is null")
	}

	idQuery = idQuery.Distinct().Select("customer.ext_id").
		Offset(pager.GetOffset()).Limit(pager.GetLimit())

	err := idQuery.Pluck("customer.ext_id", &customerIDs).Error
	if err != nil {
		err = errors.WithStack(err)
		log.Sugar.Errorw("Customer ID query failed", "error", err, "extCorpID", extCorpID)
		return nil, 0, err
	}

	// Now fetch full customer records for the IDs we found
	if len(customerIDs) > 0 {
		err = DB.Where("ext_id IN (?)", customerIDs).
			Preload("Staffs").
			Preload("Staffs.CustomerStaffTags").
			Find(&customers).Error
		if err != nil {
			err = errors.WithStack(err)
			log.Sugar.Errorw("Customer query failed", "error", err, "extCorpID", extCorpID)
			return nil, 0, err
		}
	}

	log.Sugar.Infow("Customer query completed",
		"foundCustomers", len(customers),
		"totalCount", total,
		"extCorpID", extCorpID)

	// If no customers found with relationships, try direct customer query
	if total == 0 {
		log.Sugar.Infow("No customers found with relationships, trying direct customer query", "extCorpID", extCorpID)

		directDB := DB.Where("ext_corp_id = ?", extCorpID)
		if req.Name != "" {
			directDB = directDB.Where("name like ?", req.Name+"%")
		}
		if req.Gender != 0 {
			directDB = directDB.Where("gender = ?", req.Gender)
		}
		if req.Type != 0 {
			directDB = directDB.Where("type = ?", req.Type)
		}

		var directTotal int64
		if err := directDB.Model(&Customer{}).Count(&directTotal).Error; err != nil {
			log.Sugar.Errorw("Direct customer count failed", "error", err)
		} else {
			log.Sugar.Infow("Direct customer query found", "total", directTotal, "extCorpID", extCorpID)

			if directTotal > 0 {
				directDB = directDB.Offset(pager.GetOffset()).Limit(pager.GetLimit()).
					Preload("Staffs").
					Preload("Staffs.CustomerStaffTags")

				if err := directDB.Find(&customers).Error; err != nil {
					log.Sugar.Errorw("Direct customer query failed", "error", err)
				} else {
					total = directTotal
					log.Sugar.Infow("Using direct customer query results", "count", len(customers))
				}
			}
		}
	}

	return customers, total, nil
}

func (o Customer) GetMassMsg(missionID string) (*MassMsg, error) {
	msg := &MassMsg{}
	err := DB.Model(&MassMsg{}).Where("id = ?", missionID).First(&msg).Error
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (o Customer) GetByExtID(
	ExtCustomerID string, extStaffIDs []string, withStaffRelation bool) (customer Customer, err error) {

	db := DB.Model(&Customer{})
	if withStaffRelation {
		if extStaffIDs != nil {
			db = db.Preload("Staffs", "ext_staff_id IN (?)", extStaffIDs).Preload("Staffs.CustomerStaffTags")
		} else {
			db = db.Preload("Staffs").Preload("Staffs.CustomerStaffTags")
		}
	}
	err = db.Where("ext_id = ? ", ExtCustomerID).First(&customer).Error
	if err != nil {
		err = errors.Wrap(err, "Get customer by id failed")
		return customer, err
	}
	return customer, nil
}

type CustomerSummary struct {
	CorpName               string `json:"corp_name"`
	TotalStaffsNum         int64  `json:"total_staffs_num"`
	TotalCustomersNum      int64  `json:"total_customers_num"`
	TodayCustomersIncrease int64  `json:"today_customers_increase"`
	TodayCustomersDecrease int64  `json:"today_customers_decrease"`
	TotalGroupsNum         int64  `json:"total_groups_num"`
	TodayGroupsIncrease    int64  `json:"today_groups_increase"`
	TodayGroupsDecrease    int64  `json:"today_groups_decrease"`
}
