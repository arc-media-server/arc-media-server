package org.arcmediaserver.tv.ui.theme

import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.tv.material3.LocalContentColor
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Typography
import androidx.tv.material3.darkColorScheme

/**
 * Arc's colours: the web app's dark tokens (web/src/index.css). Arc is dark only, as the owner
 * chose. The brand accent is defined here and nowhere else in code: change Accent, AccentStrong and
 * AccentLight to re-colour the app (the logo drawables and TV banner carry the same violet).
 */
object ArcColors {
    val Background = Color(0xFF0B0B10)
    val Surface = Color(0xFF15151C)
    val Surface2 = Color(0xFF1F1F28)
    val Surface3 = Color(0xFF2A2A35)
    val Foreground = Color(0xFFF5F5F7)
    val Muted = Color(0xFFA1A1AD)
    val Subtle = Color(0xFF6B6B78)
    val Accent = Color(0xFF9D8CFF)
    val AccentStrong = Color(0xFF7C68FF)
    val AccentLight = Color(0xFFC4B5FD)
    val Warning = Color(0xFFFFB547)
    val Error = Color(0xFFFF6B6B)

    val AccentGradient = Brush.linearGradient(listOf(AccentLight, AccentStrong))

    /** A profile's colour (web/src/components/Avatar.tsx), as a gradient's two ends. */
    fun profile(color: String): List<Color> = when (color) {
        "blue" -> listOf(Color(0xFF38BDF8), Color(0xFF2563EB))
        "teal" -> listOf(Color(0xFF5EEAD4), Color(0xFF0891B2))
        "green" -> listOf(Color(0xFF34D399), Color(0xFF15803D))
        "amber" -> listOf(Color(0xFFFCD34D), Color(0xFFCA8A04))
        "orange" -> listOf(Color(0xFFFB923C), Color(0xFFDC2626))
        "rose" -> listOf(Color(0xFFF472B6), Color(0xFFE11D48))
        "slate" -> listOf(Color(0xFF94A3B8), Color(0xFF334155))
        else -> listOf(Color(0xFFA78BFA), Color(0xFF4F46E5))
    }
}

private val scheme = darkColorScheme(
    primary = ArcColors.Accent,
    onPrimary = Color.White,
    primaryContainer = ArcColors.AccentStrong,
    onPrimaryContainer = Color.White,
    secondary = ArcColors.AccentLight,
    background = ArcColors.Background,
    onBackground = ArcColors.Foreground,
    surface = ArcColors.Surface,
    onSurface = ArcColors.Foreground,
    surfaceVariant = ArcColors.Surface2,
    onSurfaceVariant = ArcColors.Muted,
    inverseSurface = ArcColors.Foreground,
    inverseOnSurface = ArcColors.Background,
    border = ArcColors.Surface3,
    error = ArcColors.Error,
)

// Sizes for a 10-foot screen (the TV's 960x540dp canvas).
private val typography = Typography(
    displayMedium = TextStyle(fontSize = 44.sp, lineHeight = 52.sp, fontWeight = FontWeight.SemiBold, letterSpacing = (-0.5).sp),
    displaySmall = TextStyle(fontSize = 36.sp, lineHeight = 44.sp, fontWeight = FontWeight.SemiBold),
    headlineMedium = TextStyle(fontSize = 28.sp, lineHeight = 36.sp, fontWeight = FontWeight.SemiBold),
    headlineSmall = TextStyle(fontSize = 22.sp, lineHeight = 30.sp, fontWeight = FontWeight.SemiBold),
    titleLarge = TextStyle(fontSize = 20.sp, lineHeight = 28.sp, fontWeight = FontWeight.SemiBold),
    titleMedium = TextStyle(fontSize = 16.sp, lineHeight = 22.sp, fontWeight = FontWeight.Medium),
    titleSmall = TextStyle(fontSize = 14.sp, lineHeight = 20.sp, fontWeight = FontWeight.Medium),
    bodyLarge = TextStyle(fontSize = 16.sp, lineHeight = 24.sp),
    bodyMedium = TextStyle(fontSize = 14.sp, lineHeight = 20.sp),
    bodySmall = TextStyle(fontSize = 12.sp, lineHeight = 16.sp),
    labelLarge = TextStyle(fontSize = 14.sp, lineHeight = 20.sp, fontWeight = FontWeight.Medium),
    labelMedium = TextStyle(fontSize = 12.sp, lineHeight = 16.sp, fontWeight = FontWeight.Medium),
)

@Composable
fun ArcTheme(content: @Composable () -> Unit) {
    MaterialTheme(colorScheme = scheme, typography = typography) {
        // Text outside a Surface takes the page's colour rather than black.
        CompositionLocalProvider(LocalContentColor provides ArcColors.Foreground, content = content)
    }
}
