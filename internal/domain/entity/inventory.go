package entity

// Product representa un producto en el catálogo maestro.
type Product struct {
	ID      int
	Barcode string // Código de barras único para identificar el producto.
	Name    string // Nombre descriptivo del producto.
}

// Session representa una sesión de conteo de inventario (ej. Almacén 1).
type Session struct {
	ID        int
	Name      string // Nombre de la sesión dado por el usuario.
	CreatedAt string // Fecha y hora de creación en formato ISO8601.
}

// Record representa un registro de conteo individual dentro de una sesión.
type Record struct {
	ID        int
	SessionID int    // ID de la sesión a la que pertenece este registro.
	Barcode   string // Código de barras del producto contado.
	Name      string // Nombre del producto (unido desde el catálogo maestro).
	Quantity  int    // Delta de cantidad para este evento (positivo = entrada, negativo = venta).
	Source    string // Origen del registro: "SCAN", "LOYVERSE_SALE", "LOYVERSE_REFUND".
	CreatedAt string // Fecha y hora del evento en formato ISO8601.
}

// LoyverseEvent representa una venta o devolución reportada por Loyverse.
type LoyverseEvent struct {
	ID        int
	SessionID int
	Name      string // Nombre del producto (resuelto por el servicio).
	GroupName string // Nombre del grupo al que pertenece el producto (si aplica).
	Quantity  int    // Cantidad vendida (negativo) o devuelta (positiva).
	Source    string // "LOYVERSE_SALE" o "LOYVERSE_REFUND".
	CreatedAt string // Fecha del ticket en formato ISO8601.
}

// SessionTotals representa el total contado por producto en una sesión.
type SessionTotals struct {
	Barcode  string
	Name     string
	Quantity int
}

// CustomGroup representa un grupo personalizado de productos para descuentos de Loyverse.
type CustomGroup struct {
	ID         int
	GroupName  string // Nombre descriptivo del grupo.
	ProductIDs []int  // IDs de los productos que pertenecen a este grupo.
}
