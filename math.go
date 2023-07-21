package main

func euclideanDistance(a, b Color) float64 {
	return float64((a.R-b.R)*(a.R-b.R) + (a.G-b.G)*(a.G-b.G) + (a.B-b.B)*(a.B-b.B))
}
