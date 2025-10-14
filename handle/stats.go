package handle

import (
    "fmt"
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

// -------------------- Utils --------------------

func getDB(c *gin.Context) *gorm.DB {
    v, ok := c.Get("db")
    if !ok || v == nil {
        c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "db not in context"})
        return nil
    }
    return v.(*gorm.DB)
}

// -------------------- Page HTML --------------------

func StatsPage(c *gin.Context) {
    c.HTML(http.StatusOK, "stats.html", gin.H{
        "title": "Statistics",
    })
}

// -------------------- Résumé --------------------

type Summary struct {
    TotalTickets         int64   `json:"total_tickets"`
    OpenTickets          int64   `json:"open_tickets"`
    ClosedTickets        int64   `json:"closed_tickets"`
    AvgResolutionMinutes float64 `json:"avg_resolution_minutes"` // attendu par le front
}

func StatsSummary(c *gin.Context) {
    db := getDB(c)
    if db == nil {
        return
    }

    // Compteurs
    var total, open, closed int64
    row := db.Raw(`
        SELECT
          COUNT(*) AS total_tickets,
          SUM(CASE WHEN state IN ('open','in_progress') THEN 1 ELSE 0 END) AS open_tickets,
          SUM(CASE WHEN state = 'closed' THEN 1 ELSE 0 END) AS closed_tickets
        FROM tickets
    `).Row()
    _ = row.Scan(&total, &open, &closed)

    // Moyenne de résolution (tickets fermés uniquement)
    var avgRes float64
    row2 := db.Raw(`
        SELECT IFNULL(AVG( (julianday(closed_at) - julianday(created_at)) * 24.0 * 60.0 ), 0)
        FROM tickets
        WHERE closed_at IS NOT NULL
    `).Row()
    _ = row2.Scan(&avgRes)

    c.JSON(http.StatusOK, Summary{
        TotalTickets:         total,
        OpenTickets:          open,
        ClosedTickets:        closed,
        AvgResolutionMinutes: avgRes,
    })
}

// -------------------- Time series --------------------

type TimePoint struct {
    X string `json:"x"` // ex: "2025-10-14"
    Y int64  `json:"y"`
}

func StatsTimeSeries(c *gin.Context) {
    db := getDB(c)
    if db == nil {
        return
    }

    period := c.DefaultQuery("period", "day") // day | week | month
    typ := c.DefaultQuery("type", "created")  // created | closed
    limStr := c.DefaultQuery("limit", "")

    dateCol := "created_at"
    if typ == "closed" {
        dateCol = "closed_at"
    }

    // Bucket SQLite vers date ISO compatible Chart.js
    var bucketExpr string
    var defaultLimit int
    switch period {
    case "day":
        bucketExpr = fmt.Sprintf("strftime('%%Y-%%m-%%d', %s)", dateCol) // 2025-10-14
        defaultLimit = 30
    case "week":
        // lundi de la semaine de dateCol
        bucketExpr = fmt.Sprintf("date(%s, 'weekday 1', '-7 days')", dateCol)
        defaultLimit = 26
    case "month":
        bucketExpr = fmt.Sprintf("strftime('%%Y-%%m-01', %s)", dateCol) // 1er du mois
        defaultLimit = 12
    default:
        bucketExpr = fmt.Sprintf("strftime('%%Y-%%m-%%d', %s)", dateCol)
        defaultLimit = 30
    }

    limit := defaultLimit
    if limStr != "" {
        if v, err := strconv.Atoi(limStr); err == nil && v > 0 {
            limit = v
        }
    }

    query := fmt.Sprintf(`
        SELECT %s AS x, COUNT(*) AS y
        FROM tickets
        WHERE %s IS NOT NULL
        GROUP BY x
        ORDER BY x
        LIMIT %d
    `, bucketExpr, dateCol, limit)

    var rows []TimePoint
    if err := db.Raw(query).Scan(&rows).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed", "details": err.Error()})
        return
    }
    c.JSON(http.StatusOK, rows)
}

// -------------------- Top utilisateurs --------------------

type UserCount struct {
    UserID uint   `json:"user_id"`
    Name   string `json:"name"`
    Count  int64  `json:"count"`
}

// "created" = créateurs de tickets / "closed" = tickets fermés par utilisateur si le schéma le permet.
// Ici, on groupe par u.username = t.user, faute de colonne closed_by.
func StatsByUser(c *gin.Context) {
    db := getDB(c)
    if db == nil {
        return
    }

    typ := c.DefaultQuery("type", "created") // "created" | "closed"
    limStr := c.DefaultQuery("limit", "10")
    lim, err := strconv.Atoi(limStr)
    if err != nil || lim <= 0 {
        lim = 10
    }

    where := "t.created_at IS NOT NULL"
    if typ == "closed" {
        where = "t.closed_at IS NOT NULL"
    }

    query := fmt.Sprintf(`
        SELECT u.id AS user_id, u.username AS name, COUNT(*) AS count
        FROM tickets t
        JOIN users u ON u.username = t.user
        WHERE %s
        GROUP BY u.id, u.username
        ORDER BY count DESC
        LIMIT %d
    `, where, lim)

    var rows []UserCount
    if err := db.Raw(query).Scan(&rows).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed", "details": err.Error()})
        return
    }
    c.JSON(http.StatusOK, rows)
}
