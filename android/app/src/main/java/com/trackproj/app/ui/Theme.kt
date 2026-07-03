package com.trackproj.app.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

// Softer dark palette to match the web dashboard. Lower contrast than the
// Material default (which uses near-black backgrounds and near-white text).
private val DarkColors = darkColorScheme(
    primary       = Color(0xFF9A8CFF),
    onPrimary     = Color(0xFFFFFFFF),
    primaryContainer   = Color(0xFF2A2547),
    onPrimaryContainer = Color(0xFFE0D8FF),
    secondary     = Color(0xFFA0A5C4),
    onSecondary   = Color(0xFF14161F),
    background    = Color(0xFF0F131C),
    onBackground  = Color(0xFFC9CEDC),
    surface       = Color(0xFF161B27),
    onSurface     = Color(0xFFC9CEDC),
    surfaceVariant   = Color(0xFF1F2532),
    onSurfaceVariant = Color(0xFF98A0B4),
    outline       = Color(0xFF3A4256),
    outlineVariant = Color(0xFF262C3B),
    error         = Color(0xFFF08A8A),
    onError       = Color(0xFF1A1010),
)

private val LightColors = lightColorScheme(
    primary   = Color(0xFF6E5CE0),
    onPrimary = Color.White,
)

@Composable
fun TrackProjTheme(content: @Composable () -> Unit) {
    val colors = if (isSystemInDarkTheme()) DarkColors else LightColors
    MaterialTheme(colorScheme = colors, content = content)
}
