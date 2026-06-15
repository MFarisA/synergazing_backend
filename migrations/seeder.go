package migrations

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"synergazing.com/synergazing/model"
)

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func copySeederImage(srcName, dstPath string) {
	src := filepath.Join("picture-seeder", srcName)
	if _, err := os.Stat(src); os.IsNotExist(err) {
		log.Printf("[Warning] File gambar seeder tidak ditemukan: %s", src)
		return
	}
	if err := copyFile(src, dstPath); err != nil {
		log.Printf("[Warning] Gagal menyalin gambar seeder %s ke %s: %v", src, dstPath, err)
	} else {
		log.Printf("Berhasil menyalin gambar seeder: %s -> %s", srcName, dstPath)
	}
}

// SeedDatabase mempopulasikan seluruh tabel database dengan data realistis berbahasa Indonesia secara idempotent.
func SeedDatabase(db *gorm.DB) {
	log.Println("=== MEMULAI SEEDING DATABASE ===")

	// Salin gambar seeder dari folder picture-seeder
	log.Println("Copying seeder images to storage...")
	copySeederImage("Screenshot 2026-01-24 at 08.51.41.png", "storage/profiles/budi.png")
	copySeederImage("Screenshot 2026-01-24 at 08.51.39.png", "storage/profiles/rina.png")
	copySeederImage("Screenshot 2026-01-24 at 08.52.01.png", "storage/profiles/andi.png")
	copySeederImage("Screenshot 2026-01-24 at 23.43.30.png", "storage/profiles/siti.png")
	copySeederImage("Screenshot 2026-01-21 at 05.11.33.png", "storage/posts/project_banjir.png")
	copySeederImage("Screenshot 2026-01-24 at 08.51.39 (2).png", "storage/posts/project_koperasi.png")

	// 1. Seed Permissions
	log.Println("Seeding Permissions...")
	var permissions = []model.Permission{
		{Name: "Create Project", Slug: "create-project", Group: "Project"},
		{Name: "Edit Project", Slug: "edit-project", Group: "Project"},
		{Name: "Delete Project", Slug: "delete-project", Group: "Project"},
		{Name: "Manage Members", Slug: "manage-members", Group: "Project"},
		{Name: "Send Messages", Slug: "send-messages", Group: "Chat"},
		{Name: "Read Messages", Slug: "read-messages", Group: "Chat"},
		{Name: "Manage Users", Slug: "manage-users", Group: "User"},
	}
	var seededPermissions []*model.Permission
	for i := range permissions {
		var p model.Permission
		if err := db.Where("slug = ?", permissions[i].Slug).First(&p).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				p = permissions[i]
				if err := db.Create(&p).Error; err != nil {
					log.Fatalf("Gagal seed permission %s: %v", p.Name, err)
				}
			} else {
				log.Fatalf("Gagal kueri permission %s: %v", permissions[i].Name, err)
			}
		}
		seededPermissions = append(seededPermissions, &p)
	}

	// 2. Seed Roles
	log.Println("Seeding Roles...")
	roleAdmin := model.Role{Name: "Administrator", Description: "Administrator sistem dengan akses penuh"}
	roleOwner := model.Role{Name: "Project Owner", Description: "Pemilik proyek yang dapat mengelola anggota dan linimasa"}
	roleCollab := model.Role{Name: "Collaborator", Description: "Kolaborator yang dapat bergabung dalam proyek"}

	rolesToSeed := []*model.Role{&roleAdmin, &roleOwner, &roleCollab}
	for _, role := range rolesToSeed {
		var r model.Role
		if err := db.Where("name = ?", role.Name).First(&r).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				r = *role
				if err := db.Create(&r).Error; err != nil {
					log.Fatalf("Gagal seed role %s: %v", role.Name, err)
				}
				*role = r
			} else {
				log.Fatalf("Gagal kueri role %s: %v", role.Name, err)
			}
		} else {
			*role = r
		}
	}

	// Hubungkan Permission ke Role
	log.Println("Associating Permissions to Roles...")
	// Admin memiliki semua akses
	if err := db.Model(&roleAdmin).Association("Permissions").Replace(seededPermissions); err != nil {
		log.Fatalf("Gagal menautkan permission ke Admin: %v", err)
	}

	// Project Owner memiliki hak kelola proyek dan pesan
	var ownerPermissions []*model.Permission
	for _, p := range seededPermissions {
		if p.Group == "Project" || p.Group == "Chat" {
			ownerPermissions = append(ownerPermissions, p)
		}
	}
	if err := db.Model(&roleOwner).Association("Permissions").Replace(ownerPermissions); err != nil {
		log.Fatalf("Gagal menautkan permission ke Project Owner: %v", err)
	}

	// Collaborator memiliki hak kueri pesan
	var collabPermissions []*model.Permission
	for _, p := range seededPermissions {
		if p.Group == "Chat" {
			collabPermissions = append(collabPermissions, p)
		}
	}
	if err := db.Model(&roleCollab).Association("Permissions").Replace(collabPermissions); err != nil {
		log.Fatalf("Gagal menautkan permission ke Collaborator: %v", err)
	}

	// 3. Seed Skills
	log.Println("Seeding Skills...")
	var skillList = []string{
		"Go", "Python", "JavaScript", "TypeScript", "React", "Vue", "Docker",
		"Kubernetes", "PostgreSQL", "MongoDB", "Figma", "UI/UX Design",
		"Product Management", "Scrum", "IoT Integration", "Tailwind CSS",
	}
	skillMap := make(map[string]uint)
	for _, name := range skillList {
		var s model.Skill
		if err := db.Where("LOWER(name) = LOWER(?)", name).First(&s).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				s = model.Skill{Name: name}
				if err := db.Create(&s).Error; err != nil {
					log.Fatalf("Gagal seed skill %s: %v", name, err)
				}
			} else {
				log.Fatalf("Gagal kueri skill %s: %v", name, err)
			}
		}
		skillMap[name] = s.ID
	}

	// 4. Seed Tags
	log.Println("Seeding Tags...")
	var tagList = []string{
		"Web Development", "Mobile Development", "DevOps", "Open Source",
		"Design System", "Artificial Intelligence", "Community", "IoT Project",
	}
	tagMap := make(map[string]uint)
	for _, name := range tagList {
		var t model.Tag
		if err := db.Where("LOWER(name) = LOWER(?)", name).First(&t).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				t = model.Tag{Name: name}
				if err := db.Create(&t).Error; err != nil {
					log.Fatalf("Gagal seed tag %s: %v", name, err)
				}
			} else {
				log.Fatalf("Gagal kueri tag %s: %v", name, err)
			}
		}
		tagMap[name] = t.ID
	}

	// 5. Seed Benefits
	log.Println("Seeding Benefits...")
	var benefitList = []string{
		"Sertifikat Kolaborasi", "Jejaring Profesional (Network)", "Bimbingan Mentor Senior",
		"Portofolio Kerja Nyata", "Waktu Kerja Fleksibel / Remote",
	}
	benefitMap := make(map[string]uint)
	for _, name := range benefitList {
		var b model.Benefit
		if err := db.Where("LOWER(name) = LOWER(?)", name).First(&b).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				b = model.Benefit{Name: name}
				if err := db.Create(&b).Error; err != nil {
					log.Fatalf("Gagal seed benefit %s: %v", name, err)
				}
			} else {
				log.Fatalf("Gagal kueri benefit %s: %v", name, err)
			}
		}
		benefitMap[name] = b.ID
	}

	// 6. Seed Timelines
	log.Println("Seeding Timelines...")
	var timelineList = []string{
		"Perencanaan & Pembagian Tugas", "Desain Antarmuka (UI/UX)",
		"Pengembangan Backend & Basis Data", "Integrasi Sistem & Frontend",
		"Pengujian Aplikasi (QA/QC)", "Peluncuran Publik & Dokumentasi",
	}
	timelineMap := make(map[string]uint)
	for _, name := range timelineList {
		var t model.Timeline
		if err := db.Where("LOWER(name) = LOWER(?)", name).First(&t).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				t = model.Timeline{Name: name}
				if err := db.Create(&t).Error; err != nil {
					log.Fatalf("Gagal seed timeline %s: %v", name, err)
				}
			} else {
				log.Fatalf("Gagal kueri timeline %s: %v", name, err)
			}
		}
		timelineMap[name] = t.ID
	}

	// 7. Seed Users (passwords hashed with bcrypt)
	log.Println("Seeding Users...")
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Gagal melakukan hashing password: %v", err)
	}

	usersData := []model.Users{
		{
			Name:                "Budi Santoso",
			Email:               "budi@example.com",
			Password:            string(hashedPassword),
			Phone:               "081234567890",
			StatusCollaboration: "ready",
			IsEmailVerified:     true,
		},
		{
			Name:                "Rina Wijaya",
			Email:               "rina@example.com",
			Password:            string(hashedPassword),
			Phone:               "082345678901",
			StatusCollaboration: "ready",
			IsEmailVerified:     true,
		},
		{
			Name:                "Andi Pratama",
			Email:               "andi@example.com",
			Password:            string(hashedPassword),
			Phone:               "083456789012",
			StatusCollaboration: "ready",
			IsEmailVerified:     true,
		},
		{
			Name:                "Siti Aminah",
			Email:               "siti@example.com",
			Password:            string(hashedPassword),
			Phone:               "084567890123",
			StatusCollaboration: "not ready",
			IsEmailVerified:     true,
		},
	}

	userMap := make(map[string]*model.Users)
	for i := range usersData {
		var u model.Users
		if err := db.Where("email = ?", usersData[i].Email).First(&u).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				u = usersData[i]
				if err := db.Create(&u).Error; err != nil {
					log.Fatalf("Gagal seed user %s: %v", u.Name, err)
				}
			} else {
				log.Fatalf("Gagal kueri user %s: %v", usersData[i].Email, err)
			}
		}
		userMap[u.Email] = &u
	}

	// Tautkan User ke Role
	log.Println("Associating Users to Roles...")
	db.Model(userMap["budi@example.com"]).Association("Role").Replace([]*model.Role{&roleCollab})
	db.Model(userMap["rina@example.com"]).Association("Role").Replace([]*model.Role{&roleOwner})
	db.Model(userMap["andi@example.com"]).Association("Role").Replace([]*model.Role{&roleCollab})
	db.Model(userMap["siti@example.com"]).Association("Role").Replace([]*model.Role{&roleAdmin})

	// 8. Seed Profiles
	log.Println("Seeding Profiles...")
	profilesData := []model.Profiles{
		{
			UserID:         userMap["budi@example.com"].ID,
			AboutMe:        "Halo, saya Budi! Pengembang Frontend yang berdedikasi tinggi dan berfokus pada pembuatan website responsif menggunakan React dan Tailwind CSS.",
			Location:       "Jakarta Barat, DKI Jakarta",
			Interests:      "Web Tech, Open Source, UI/UX Animations",
			Academic:       "S1 Teknik Informatika - Universitas Indonesia",
			WebsiteURL:     "https://budisantoso.dev",
			GithubURL:      "https://github.com/budisantoso",
			LinkedInURL:    "https://linkedin.com/in/budisantoso",
			ProfilePicture: "storage/profiles/budi.png",
		},
		{
			UserID:         userMap["rina@example.com"].ID,
			AboutMe:        "Saya Rina, desainer produk digital dengan kecintaan mendalam pada pembuatan pengalaman pengguna yang sederhana dan memikat.",
			Location:       "Bandung, Jawa Barat",
			Interests:      "Design System, Usability Testing, Figma Community",
			Academic:       "S1 Desain Komunikasi Visual - Institut Teknologi Bandung",
			WebsiteURL:     "https://rinawijaya.my.id",
			LinkedInURL:    "https://linkedin.com/in/rinawijaya",
			ProfilePicture: "storage/profiles/rina.png",
		},
		{
			UserID:         userMap["andi@example.com"].ID,
			AboutMe:        "Backend developer dengan keahlian utama di bahasa Go, PostgreSQL, dan Docker. Menyukai optimasi query database SQL.",
			Location:       "Surabaya, Jawa Timur",
			Interests:      "Sistem Terdistribusi, Clean Architecture, Server Management",
			Academic:       "S1 Sistem Informasi - Institut Teknologi Sepuluh Nopember",
			WebsiteURL:     "https://andipratama.com",
			GithubURL:      "https://github.com/andipratama",
			LinkedInURL:    "https://linkedin.com/in/andipratama",
			ProfilePicture: "storage/profiles/andi.png",
		},
		{
			UserID:         userMap["siti@example.com"].ID,
			AboutMe:        "Product manager bersertifikat Scrum Alliance dengan pengalaman mengarahkan produk perangkat lunak bernilai guna tinggi.",
			Location:       "Sleman, DI Yogyakarta",
			Interests:      "Product Management, Agile, Agile Transformation",
			Academic:       "S1 Ilmu Komputer - Universitas Gadjah Mada",
			LinkedInURL:    "https://linkedin.com/in/sitiaminah",
			ProfilePicture: "storage/profiles/siti.png",
		},
	}

	for i := range profilesData {
		var prof model.Profiles
		if err := db.Where("user_id = ?", profilesData[i].UserID).First(&prof).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				prof = profilesData[i]
				if err := db.Create(&prof).Error; err != nil {
					log.Fatalf("Gagal seed profile user ID %d: %v", prof.UserID, err)
				}
			} else {
				log.Fatalf("Gagal kueri profile: %v", err)
			}
		}
	}

	// 9. Seed User Skills
	log.Println("Seeding User Skills...")
	userSkillsData := []model.UserSkill{
		{UserID: userMap["budi@example.com"].ID, SkillID: skillMap["JavaScript"], Proficiency: 90},
		{UserID: userMap["budi@example.com"].ID, SkillID: skillMap["TypeScript"], Proficiency: 80},
		{UserID: userMap["budi@example.com"].ID, SkillID: skillMap["React"], Proficiency: 85},
		{UserID: userMap["budi@example.com"].ID, SkillID: skillMap["Tailwind CSS"], Proficiency: 88},
		{UserID: userMap["rina@example.com"].ID, SkillID: skillMap["Figma"], Proficiency: 95},
		{UserID: userMap["rina@example.com"].ID, SkillID: skillMap["UI/UX Design"], Proficiency: 90},
		{UserID: userMap["andi@example.com"].ID, SkillID: skillMap["Go"], Proficiency: 92},
		{UserID: userMap["andi@example.com"].ID, SkillID: skillMap["PostgreSQL"], Proficiency: 85},
		{UserID: userMap["andi@example.com"].ID, SkillID: skillMap["Docker"], Proficiency: 75},
		{UserID: userMap["siti@example.com"].ID, SkillID: skillMap["Product Management"], Proficiency: 90},
		{UserID: userMap["siti@example.com"].ID, SkillID: skillMap["Scrum"], Proficiency: 85},
	}
	for i := range userSkillsData {
		var us model.UserSkill
		if err := db.Where("user_id = ? AND skill_id = ?", userSkillsData[i].UserID, userSkillsData[i].SkillID).First(&us).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				us = userSkillsData[i]
				if err := db.Create(&us).Error; err != nil {
					log.Fatalf("Gagal seed user skill: %v", err)
				}
			} else {
				log.Fatalf("Gagal kueri user skill: %v", err)
			}
		}
	}

	// 10. Seed Projects (Complete 5 Stages)
	log.Println("Seeding Projects...")
	projectsData := []model.Project{
		{
			CreatorID:            userMap["rina@example.com"].ID,
			Title:                "Sistem Deteksi Banjir Cerdas Jakarta",
			ProjectType:          "Web & IoT Development",
			Description:          "Proyek kolaboratif berskala kota untuk membangun dashboard monitoring real-time tinggi muka air pintu air dan sistem peringatan dini otomatis berbasis data sensor IoT di daerah rawan banjir Jakarta.",
			Status:               "published",
			CompletionStage:      5,
			Duration:             "3 Bulan",
			TotalTeam:            5,
			StartDate:            time.Now().AddDate(0, 0, 7),
			EndDate:              time.Now().AddDate(0, 3, 7),
			Location:             "Hybrid (Jakarta Barat / Remote)",
			Budget:               "Rp 15.000.000 (Pendanaan Hibah)",
			RegistrationDeadline: time.Now().AddDate(0, 0, 5),
			TimeCommitment:       "15-20 Jam per Minggu",
			PictureURL:           "storage/posts/project_banjir.png",
		},
		{
			CreatorID:            userMap["andi@example.com"].ID,
			Title:                "Aplikasi E-Commerce Koperasi Desa",
			ProjectType:          "Mobile App Development",
			Description:          "Pengembangan aplikasi seluler (Android/iOS) koperasi syariah pedesaan untuk memudahkan manajemen produk lokal unggulan, pembukuan keuangan, dan transaksi bagi hasil antar petani secara transparan.",
			Status:               "published",
			CompletionStage:      5,
			Duration:             "4 Bulan",
			TotalTeam:            4,
			StartDate:            time.Now().AddDate(0, 0, 14),
			EndDate:              time.Now().AddDate(0, 4, 14),
			Location:             "Remote",
			Budget:               "Rp 20.000.000",
			RegistrationDeadline: time.Now().AddDate(0, 0, 10),
			TimeCommitment:       "10-15 Jam per Minggu",
			PictureURL:           "storage/posts/project_koperasi.png",
		},
	}

	projectMap := make(map[string]*model.Project)
	for i := range projectsData {
		var prj model.Project
		if err := db.Where("title = ?", projectsData[i].Title).First(&prj).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				prj = projectsData[i]
				if err := db.Create(&prj).Error; err != nil {
					log.Fatalf("Gagal seed project %s: %v", prj.Title, err)
				}
			} else {
				log.Fatalf("Gagal kueri project %s: %v", projectsData[i].Title, err)
			}
		}
		projectMap[prj.Title] = &prj
	}

	// Tautkan Project ke Required Skills
	log.Println("Seeding Project Required Skills...")
	proj1Skills := []model.ProjectRequiredSkill{
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, SkillID: skillMap["Go"]},
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, SkillID: skillMap["React"]},
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, SkillID: skillMap["PostgreSQL"]},
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, SkillID: skillMap["UI/UX Design"]},
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, SkillID: skillMap["IoT Integration"]},
	}
	for i := range proj1Skills {
		db.Where("project_id = ? AND skill_id = ?", proj1Skills[i].ProjectID, proj1Skills[i].SkillID).FirstOrCreate(&proj1Skills[i])
	}
	proj2Skills := []model.ProjectRequiredSkill{
		{ProjectID: projectMap["Aplikasi E-Commerce Koperasi Desa"].ID, SkillID: skillMap["TypeScript"]},
		{ProjectID: projectMap["Aplikasi E-Commerce Koperasi Desa"].ID, SkillID: skillMap["React"]},
		{ProjectID: projectMap["Aplikasi E-Commerce Koperasi Desa"].ID, SkillID: skillMap["PostgreSQL"]},
	}
	for i := range proj2Skills {
		db.Where("project_id = ? AND skill_id = ?", proj2Skills[i].ProjectID, proj2Skills[i].SkillID).FirstOrCreate(&proj2Skills[i])
	}

	// Tautkan Project ke Tags
	log.Println("Seeding Project Tags...")
	proj1Tags := []model.ProjectTag{
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, TagID: tagMap["Web Development"]},
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, TagID: tagMap["IoT Project"]},
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, TagID: tagMap["Community"]},
	}
	for i := range proj1Tags {
		db.Where("project_id = ? AND tag_id = ?", proj1Tags[i].ProjectID, proj1Tags[i].TagID).FirstOrCreate(&proj1Tags[i])
	}
	proj2Tags := []model.ProjectTag{
		{ProjectID: projectMap["Aplikasi E-Commerce Koperasi Desa"].ID, TagID: tagMap["Mobile Development"]},
		{ProjectID: projectMap["Aplikasi E-Commerce Koperasi Desa"].ID, TagID: tagMap["Design System"]},
	}
	for i := range proj2Tags {
		db.Where("project_id = ? AND tag_id = ?", proj2Tags[i].ProjectID, proj2Tags[i].TagID).FirstOrCreate(&proj2Tags[i])
	}

	// Tautkan Project ke Benefits
	log.Println("Seeding Project Benefits...")
	proj1Benefits := []model.ProjectBenefit{
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, BenefitID: benefitMap["Sertifikat Kolaborasi"]},
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, BenefitID: benefitMap["Jejaring Profesional (Network)"]},
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, BenefitID: benefitMap["Portofolio Kerja Nyata"]},
	}
	for i := range proj1Benefits {
		db.Where("project_id = ? AND benefit_id = ?", proj1Benefits[i].ProjectID, proj1Benefits[i].BenefitID).FirstOrCreate(&proj1Benefits[i])
	}
	proj2Benefits := []model.ProjectBenefit{
		{ProjectID: projectMap["Aplikasi E-Commerce Koperasi Desa"].ID, BenefitID: benefitMap["Portofolio Kerja Nyata"]},
		{ProjectID: projectMap["Aplikasi E-Commerce Koperasi Desa"].ID, BenefitID: benefitMap["Waktu Kerja Fleksibel / Remote"]},
	}
	for i := range proj2Benefits {
		db.Where("project_id = ? AND benefit_id = ?", proj2Benefits[i].ProjectID, proj2Benefits[i].BenefitID).FirstOrCreate(&proj2Benefits[i])
	}

	// Tautkan Project ke Timelines
	log.Println("Seeding Project Timelines...")
	proj1Timelines := []model.ProjectTimeline{
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, TimelineID: timelineMap["Perencanaan & Pembagian Tugas"], TimelineStatus: "done"},
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, TimelineID: timelineMap["Desain Antarmuka (UI/UX)"], TimelineStatus: "in-progress"},
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, TimelineID: timelineMap["Pengembangan Backend & Basis Data"], TimelineStatus: "not-started"},
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, TimelineID: timelineMap["Integrasi Sistem & Frontend"], TimelineStatus: "not-started"},
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, TimelineID: timelineMap["Pengujian Aplikasi (QA/QC)"], TimelineStatus: "not-started"},
	}
	for i := range proj1Timelines {
		db.Where("project_id = ? AND timeline_id = ?", proj1Timelines[i].ProjectID, proj1Timelines[i].TimelineID).FirstOrCreate(&proj1Timelines[i])
	}
	proj2Timelines := []model.ProjectTimeline{
		{ProjectID: projectMap["Aplikasi E-Commerce Koperasi Desa"].ID, TimelineID: timelineMap["Perencanaan & Pembagian Tugas"], TimelineStatus: "in-progress"},
		{ProjectID: projectMap["Aplikasi E-Commerce Koperasi Desa"].ID, TimelineID: timelineMap["Desain Antarmuka (UI/UX)"], TimelineStatus: "not-started"},
		{ProjectID: projectMap["Aplikasi E-Commerce Koperasi Desa"].ID, TimelineID: timelineMap["Pengembangan Backend & Basis Data"], TimelineStatus: "not-started"},
		{ProjectID: projectMap["Aplikasi E-Commerce Koperasi Desa"].ID, TimelineID: timelineMap["Pengujian Aplikasi (QA/QC)"], TimelineStatus: "not-started"},
	}
	for i := range proj2Timelines {
		db.Where("project_id = ? AND timeline_id = ?", proj2Timelines[i].ProjectID, proj2Timelines[i].TimelineID).FirstOrCreate(&proj2Timelines[i])
	}

	// Seed Project Conditions
	log.Println("Seeding Project Conditions...")
	var conditions = []model.ProjectCondition{
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, Description: "Berdomisili di area Jabodetabek untuk kemudahan koordinasi hybrid."},
		{ProjectID: projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID, Description: "Memiliki ketertarikan tinggi pada krisis iklim dan mitigasi bencana bencana."},
		{ProjectID: projectMap["Aplikasi E-Commerce Koperasi Desa"].ID, Description: "Berpengalaman setidaknya 1 tahun merancang skema database PostgreSQL."},
	}
	for i := range conditions {
		db.Where("project_id = ? AND description = ?", conditions[i].ProjectID, conditions[i].Description).FirstOrCreate(&conditions[i])
	}

	// 11. Seed Project Roles
	log.Println("Seeding Project Roles...")
	rolesData := []model.ProjectRole{
		{
			ProjectID:      projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID,
			Name:           "UI/UX Designer",
			SlotsAvailable: 1,
			Description:    "Bertanggung jawab merancang prototipe interaktif dashboard visualisasi banjir, peta stasiun pompa, dan user flow pelaporan warga.",
		},
		{
			ProjectID:      projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID,
			Name:           "Frontend Developer",
			SlotsAvailable: 1,
			Description:    "Bertanggung jawab membangun antarmuka web dashboard berbasis React, chart grafik muka air, dan integrasi peta Leaflet.js.",
		},
		{
			ProjectID:      projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID,
			Name:           "Backend Engineer",
			SlotsAvailable: 1,
			Description:    "Bertanggung jawab membangun RESTful API backend menggunakan Go, mengintegrasikan data sensor IoT, dan menyimpannya ke PostgreSQL.",
		},
		{
			ProjectID:      projectMap["Aplikasi E-Commerce Koperasi Desa"].ID,
			Name:           "React Native Developer",
			SlotsAvailable: 2,
			Description:    "Bertanggung jawab membangun aplikasi mobile multiplatform Android/iOS dari nol menggunakan kerangka kerja React Native.",
		},
		{
			ProjectID:      projectMap["Aplikasi E-Commerce Koperasi Desa"].ID,
			Name:           "Database Administrator",
			SlotsAvailable: 1,
			Description:    "Bertanggung jawab merancang, mengamankan, dan melakukan normalisasi tabel transaksi keuangan simpan pinjam anggota koperasi.",
		},
	}

	projRoleMap := make(map[string]uint)
	for i := range rolesData {
		var pr model.ProjectRole
		if err := db.Where("project_id = ? AND name = ?", rolesData[i].ProjectID, rolesData[i].Name).First(&pr).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				pr = rolesData[i]
				if err := db.Create(&pr).Error; err != nil {
					log.Fatalf("Gagal seed project role %s: %v", pr.Name, err)
				}
			} else {
				log.Fatalf("Gagal kueri project role: %v", err)
			}
		}
		// Gunakan gabungan ProjectID_RoleName sebagai kunci map unik
		key := string(rune(pr.ProjectID)) + "_" + pr.Name
		projRoleMap[key] = pr.ID
	}

	// Seed Project Role Skills (Hubungkan ke Skill)
	log.Println("Seeding Project Role Skills...")
	roleSkillsData := []model.ProjectRoleSkill{
		{ProjectRoleID: projRoleMap[string(rune(projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID))+"_UI/UX Designer"], SkillID: skillMap["Figma"]},
		{ProjectRoleID: projRoleMap[string(rune(projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID))+"_UI/UX Designer"], SkillID: skillMap["UI/UX Design"]},
		{ProjectRoleID: projRoleMap[string(rune(projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID))+"_Frontend Developer"], SkillID: skillMap["JavaScript"]},
		{ProjectRoleID: projRoleMap[string(rune(projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID))+"_Frontend Developer"], SkillID: skillMap["React"]},
		{ProjectRoleID: projRoleMap[string(rune(projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID))+"_Backend Engineer"], SkillID: skillMap["Go"]},
		{ProjectRoleID: projRoleMap[string(rune(projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID))+"_Backend Engineer"], SkillID: skillMap["PostgreSQL"]},
		{ProjectRoleID: projRoleMap[string(rune(projectMap["Aplikasi E-Commerce Koperasi Desa"].ID))+"_React Native Developer"], SkillID: skillMap["TypeScript"]},
		{ProjectRoleID: projRoleMap[string(rune(projectMap["Aplikasi E-Commerce Koperasi Desa"].ID))+"_React Native Developer"], SkillID: skillMap["React"]},
		{ProjectRoleID: projRoleMap[string(rune(projectMap["Aplikasi E-Commerce Koperasi Desa"].ID))+"_Database Administrator"], SkillID: skillMap["PostgreSQL"]},
	}
	for i := range roleSkillsData {
		db.Where("project_role_id = ? AND skill_id = ?", roleSkillsData[i].ProjectRoleID, roleSkillsData[i].SkillID).FirstOrCreate(&roleSkillsData[i])
	}

	// 12. Seed Project Members
	log.Println("Seeding Project Members...")
	// Budi Santoso masuk ke Proyek 1 sebagai Frontend Developer
	budiRoleID := projRoleMap[string(rune(projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID))+"_Frontend Developer"]
	budiMember := model.ProjectMember{
		ProjectID:       projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID,
		UserID:          userMap["budi@example.com"].ID,
		ProjectRoleID:   budiRoleID,
		Status:          "joined",
		RoleDescription: "Mengembangkan dashboard antarmuka visualisasi sensor air pintu air.",
	}
	if err := db.Where("project_id = ? AND user_id = ?", budiMember.ProjectID, budiMember.UserID).FirstOrCreate(&budiMember).Error; err != nil {
		log.Fatalf("Gagal seed project member: %v", err)
	}

	// Tambahkan ke ProjectMemberSkill
	budiMemSkills := []model.ProjectMemberSkill{
		{ProjectMemberID: budiMember.ID, SkillID: skillMap["React"]},
		{ProjectMemberID: budiMember.ID, SkillID: skillMap["JavaScript"]},
	}
	for i := range budiMemSkills {
		db.Where("project_member_id = ? AND skill_id = ?", budiMemSkills[i].ProjectMemberID, budiMemSkills[i].SkillID).FirstOrCreate(&budiMemSkills[i])
	}

	// 13. Seed Project Applications
	log.Println("Seeding Project Applications...")
	// Andi Pratama mendaftar sebagai Backend Engineer di Proyek 1
	andiPrjRoleID := projRoleMap[string(rune(projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID))+"_Backend Engineer"]
	andiApp := model.ProjectApplication{
		ProjectID:        projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID,
		UserID:           userMap["andi@example.com"].ID,
		ProjectRoleID:    andiPrjRoleID,
		Status:           "pending",
		WhyInterested:    "Saya sangat tertarik pada penanggulangan banjir di kota Jakarta lewat pendekatan Internet of Things. Sistem pengolahan data sensor air harus cepat dan stabil.",
		SkillsExperience: "Saya telah membangun 3 buah API skala menengah menggunakan Golang, Gin framework, dan PostgreSQL selama masa kuliah dan proyek sampingan saya.",
		Contribution:     "Saya akan mengoptimasi desain database sensor air dan mengimplementasikan REST API yang handal dengan integrasi WebSocket untuk pembaruan instan ke frontend.",
		AppliedAt:        time.Now().AddDate(0, 0, -2),
	}
	db.Where("project_id = ? AND user_id = ? AND project_role_id = ?", andiApp.ProjectID, andiApp.UserID, andiApp.ProjectRoleID).FirstOrCreate(&andiApp)

	// 14. Seed Chats & Messages (Budi & Rina obrolan pengerjaan proyek)
	log.Println("Seeding Chats and Messages...")
	user1ID := userMap["budi@example.com"].ID
	user2ID := userMap["rina@example.com"].ID
	if user1ID > user2ID {
		user1ID, user2ID = user2ID, user1ID
	}

	var chat model.Chat
	if err := db.Where("user1_id = ? AND user2_id = ?", user1ID, user2ID).First(&chat).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			chat = model.Chat{User1ID: user1ID, User2ID: user2ID}
			if err := db.Create(&chat).Error; err != nil {
				log.Fatalf("Gagal seed chat: %v", err)
			}
		} else {
			log.Fatalf("Gagal kueri chat: %v", err)
		}
	}

	// Seed Messages dalam Chat
	messagesData := []model.Message{
		{
			ChatID:    chat.ID,
			SenderID:  userMap["rina@example.com"].ID,
			Content:   "Halo Budi, selamat bergabung di tim Sistem Deteksi Banjir Cerdas! Apakah kamu sudah sempat meninjau wireframe UI terbaru di Figma?",
			IsRead:    true,
			CreatedAt: time.Now().Add(-15 * time.Minute),
		},
		{
			ChatID:    chat.ID,
			SenderID:  userMap["budi@example.com"].ID,
			Content:   "Halo Mbak Rina, terima kasih banyak! Ya, saya sudah melihat rancangan di Figma. Tampilannya sangat bersih. Rencananya saya akan menggunakan Tailwind CSS dan Leaflet.js untuk rendering peta sensornya.",
			IsRead:    true,
			CreatedAt: time.Now().Add(-10 * time.Minute),
		},
		{
			ChatID:    chat.ID,
			SenderID:  userMap["rina@example.com"].ID,
			Content:   "Bagus sekali. Tolong perhatikan pewarnaan stasiun pompa pintu air agar mencerminkan status Siaga 1, 2, atau 3 sesuai standar BPBD ya.",
			IsRead:    true,
			CreatedAt: time.Now().Add(-5 * time.Minute),
		},
		{
			ChatID:    chat.ID,
			SenderID:  userMap["budi@example.com"].ID,
			Content:   "Siap Mbak, saya akan pelajari respons skema data sensor dari Backend terlebih dahulu agar sinkronisasinya pas.",
			IsRead:    false,
			CreatedAt: time.Now().Add(-2 * time.Minute),
		},
	}

	for i := range messagesData {
		var msg model.Message
		// Cari pesan dengan konten dan waktu mirip agar tidak duplikat
		if err := db.Where("chat_id = ? AND sender_id = ? AND content = ?", messagesData[i].ChatID, messagesData[i].SenderID, messagesData[i].Content).First(&msg).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				msg = messagesData[i]
				if err := db.Create(&msg).Error; err != nil {
					log.Fatalf("Gagal seed message: %v", err)
				}
			}
		}
	}

	// 15. Seed Notifications
	log.Println("Seeding Notifications...")
	proj1ID := projectMap["Sistem Deteksi Banjir Cerdas Jakarta"].ID
	data1, _ := json.Marshal(map[string]interface{}{"project_id": proj1ID, "role": "Frontend Developer"})
	data2, _ := json.Marshal(map[string]interface{}{"project_id": proj1ID, "applicant_name": "Andi Pratama"})

	notificationsData := []model.Notification{
		{
			UserID:    userMap["budi@example.com"].ID,
			ProjectID: &proj1ID,
			Type:      "invitation_received",
			Title:     "Undangan Bergabung Proyek",
			Message:   "Rina Wijaya mengundang Anda bergabung ke proyek 'Sistem Deteksi Banjir Cerdas Jakarta' sebagai Frontend Developer",
			IsRead:    true,
			Data:      string(data1),
		},
		{
			UserID:    userMap["rina@example.com"].ID,
			ProjectID: &proj1ID,
			Type:      "user_registered",
			Title:     "Pendaftaran Masuk Baru",
			Message:   "Andi Pratama mengajukan diri untuk bergabung ke proyek 'Sistem Deteksi Banjir Cerdas Jakarta' sebagai Backend Engineer",
			IsRead:    false,
			Data:      string(data2),
		},
	}

	for i := range notificationsData {
		var n model.Notification
		if err := db.Where("user_id = ? AND title = ? AND message = ?", notificationsData[i].UserID, notificationsData[i].Title, notificationsData[i].Message).First(&n).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				n = notificationsData[i]
				if err := db.Create(&n).Error; err != nil {
					log.Fatalf("Gagal seed notification: %v", err)
				}
			}
		}
	}

	log.Println("=== SEEDING DATABASE SELESAI DENGAN SUKSES ===")
}
