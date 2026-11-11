// Shared basemap definitions so every map page renders the same tiles.
// Esri's Canvas basemaps are keyless; CARTO's free tier now watermarks tiles.
const ESRI = 'https://services.arcgisonline.com/ArcGIS/rest/services/Canvas'
const ATTRIBUTION = 'Tiles &copy; Esri &mdash; Esri, HERE, Garmin, &copy; OpenStreetMap contributors'

export const TILES = {
  dark: {
    url: `${ESRI}/World_Dark_Gray_Base/MapServer/tile/{z}/{y}/{x}`,
    attribution: `${ATTRIBUTION}, and the GIS user community`,
    maxNativeZoom: 16,
  },
  light: {
    url: `${ESRI}/World_Light_Gray_Base/MapServer/tile/{z}/{y}/{x}`,
    attribution: ATTRIBUTION,
    maxNativeZoom: 16,
  },
}
