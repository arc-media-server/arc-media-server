package org.arcmediaserver.tv

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import org.arcmediaserver.tv.ui.ArcNavHost
import org.arcmediaserver.tv.ui.theme.ArcTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            ArcTheme { ArcNavHost() }
        }
    }
}
