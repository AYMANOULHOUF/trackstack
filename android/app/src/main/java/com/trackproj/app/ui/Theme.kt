package com.trackproj.app.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

// TrackStack palette: deep green #14532D with cream #F6F1E8. The dark scheme
// lifts the brand green to a mint so interactive elements stay readable on
// the near-black, green-tinted surfaces.
private val DarkColors = darkColorScheme(
    primary       = Color(0xFF7BD3A0),
    onPrimary     = Color(0xFF072015),
    primaryContainer   = Color(0xFF1B4A34),
    onPrimaryContainer = Color(0xFFC9EBD8),
    secondary     = Color(0xFFA9C0B1),
    onSecondary   = Color(0xFF0C1A12),
    secondaryContainer   = Color(0xFF24382D),
    onSecondaryContainer = Color(0xFFC9EBD8),
    background    = Color(0xFF0D1411),
    onBackground  = Color(0xFFC9D6CD),
    surface       = Color(0xFF141D18),
    onSurface     = Color(0xFFC9D6CD),
    surfaceVariant   = Color(0xFF1D2A23),
    onSurfaceVariant = Color(0xFF98AC9F),
    outline       = Color(0xFF374A3F),
    outlineVariant = Color(0xFF24312A),
    error         = Color(0xFFF08A8A),
    onError       = Color(0xFF1A1010),
)

private val LightColors = lightColorScheme(
    primary   = Color(0xFF14532D),
    onPrimary = Color(0xFFF6F1E8),
    primaryContainer   = Color(0xFFD7EDE0),
    onPrimaryContainer = Color(0xFF0B2E1A),
    secondary = Color(0xFF4D5F54),
    onSecondary = Color(0xFFF6F1E8),
    background = Color(0xFFF6F1E8),
    onBackground = Color(0xFF14201A),
    surface = Color(0xFFFFFFFF),
    onSurface = Color(0xFF14201A),
    surfaceVariant = Color(0xFFE7EFE9),
    onSurfaceVariant = Color(0xFF4D5F54),
    outline = Color(0xFFB9C9BF),
    outlineVariant = Color(0xFFDCE6DF),
    error = Color(0xFFB3261E),
    onError = Color(0xFFFFFFFF),
)

@Composable
fun TrackProjTheme(content: @Composable () -> Unit) {
    val colors = if (isSystemInDarkTheme()) DarkColors else LightColors
    MaterialTheme(colorScheme = colors, content = content)
}
