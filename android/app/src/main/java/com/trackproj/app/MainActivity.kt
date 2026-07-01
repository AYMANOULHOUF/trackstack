package com.trackproj.app

import android.Manifest
import android.annotation.SuppressLint
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.location.LocationManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.PowerManager
import android.provider.Settings
import androidx.activity.ComponentActivity
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.DarkMode
import androidx.compose.material.icons.filled.LightMode
import androidx.compose.material.icons.filled.LocationOff
import androidx.compose.material.icons.filled.LocationOn
import androidx.compose.material.icons.filled.MyLocation
import androidx.compose.material.icons.filled.Remove
import androidx.compose.material3.Button
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.SmallFloatingActionButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalLifecycleOwner
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import com.trackproj.app.tracking.LocationTrackingService
import com.trackproj.app.auth.ApiClient
import com.trackproj.app.auth.TokenStore
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import com.trackproj.app.ui.TrackProjTheme
import org.osmdroid.config.Configuration
import org.osmdroid.tileprovider.tilesource.TileSourceFactory
import org.osmdroid.util.GeoPoint
import org.osmdroid.views.CustomZoomButtonsController
import org.osmdroid.views.MapView
import org.osmdroid.views.overlay.TilesOverlay
import org.osmdroid.views.overlay.compass.CompassOverlay
import org.osmdroid.views.overlay.compass.InternalCompassOrientationProvider
import org.osmdroid.views.overlay.Polyline
import org.osmdroid.views.overlay.ScaleBarOverlay
import org.osmdroid.views.overlay.gestures.RotationGestureOverlay
import org.osmdroid.views.overlay.mylocation.GpsMyLocationProvider
import org.osmdroid.views.overlay.mylocation.MyLocationNewOverlay

class MainActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        Configuration.getInstance().userAgentValue = packageName
        setContent {
            TrackProjTheme { TrackerScreen() }
        }
    }

    // --- imperative helpers ---

    private fun hasFineLocation() = ContextCompat.checkSelfPermission(
        this, Manifest.permission.ACCESS_FINE_LOCATION
    ) == PackageManager.PERMISSION_GRANTED

    private fun needsNotifPermission() =
        Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(
                this, Manifest.permission.POST_NOTIFICATIONS
            ) != PackageManager.PERMISSION_GRANTED

    private fun needsBackgroundLocation() =
        Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q &&
            ContextCompat.checkSelfPermission(
                this, Manifest.permission.ACCESS_BACKGROUND_LOCATION
            ) != PackageManager.PERMISSION_GRANTED

    private fun isLocationEnabled(): Boolean {
        val lm = getSystemService(LocationManager::class.java)
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            lm.isLocationEnabled
        } else {
            @Suppress("DEPRECATION")
            lm.isProviderEnabled(LocationManager.GPS_PROVIDER) ||
                lm.isProviderEnabled(LocationManager.NETWORK_PROVIDER)
        }
    }

    private fun promptEnableLocation() =
        startActivity(Intent(Settings.ACTION_LOCATION_SOURCE_SETTINGS))

    private fun isIgnoringBatteryOptimizations(): Boolean {
        val pm = getSystemService(PowerManager::class.java)
        return pm.isIgnoringBatteryOptimizations(packageName)
    }

    @SuppressLint("BatteryLife")
    private fun promptDisableBatteryOptimization() {
        startActivity(
            Intent(
                Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS,
                Uri.parse("package:$packageName")
            )
        )
    }

    private fun openAppSettings() {
        startActivity(
            Intent(
                Settings.ACTION_APPLICATION_DETAILS_SETTINGS,
                Uri.parse("package:$packageName")
            )
        )
    }

    private fun startTrackingService() {
        val i = Intent(this, LocationTrackingService::class.java).apply {
            action = LocationTrackingService.ACTION_START
        }
        ContextCompat.startForegroundService(this, i)
    }

    private fun stopTrackingService() {
        val i = Intent(this, LocationTrackingService::class.java).apply {
            action = LocationTrackingService.ACTION_STOP
        }
        startService(i)
    }

    @Composable
    private fun TrackerScreen() {
        val context = LocalContext.current
        val lifecycleOwner = LocalLifecycleOwner.current
        val tokenStore = remember { TokenStore(context) }
        var deviceReady by remember { mutableStateOf(tokenStore.hasDevice()) }
        var enrollError by remember { mutableStateOf<String?>(null) }
        var enrollAttempt by remember { mutableStateOf(0) }

        // First launch: silently enroll this phone (no login). It becomes an
        // unassigned/pending device until the admin assigns it to an org.
        LaunchedEffect(enrollAttempt) {
            if (!tokenStore.hasDevice()) {
                enrollError = null
                try {
                    withContext(Dispatchers.IO) {
                        val d = ApiClient().enroll("${Build.MANUFACTURER} ${Build.MODEL}")
                        tokenStore.deviceId = d.id
                        tokenStore.deviceToken = d.apiToken
                    }
                    deviceReady = true
                } catch (e: Exception) {
                    enrollError = e.message ?: "enrollment failed"
                }
            }
        }


        var tracking by remember { mutableStateOf(LocationTrackingService.isRunning) }
        var statusText by remember {
            mutableStateOf(if (tracking) "Tracking on" else "Ready to track")
        }
        var darkMap by remember { mutableStateOf(false) }
        var initializedTheme by remember { mutableStateOf(false) }

        // Permission gate state.
        var fineGranted by remember { mutableStateOf(hasFineLocation()) }
        var permanentlyDenied by remember { mutableStateOf(false) }

        // --- map + overlays (created once) ---
        val mapView = remember {
            MapView(context).apply {
                setTileSource(TileSourceFactory.MAPNIK)
                setMultiTouchControls(true)
                zoomController.setVisibility(CustomZoomButtonsController.Visibility.NEVER)
                controller.setZoom(16.0)
            }
        }
        val trail = remember { Polyline().apply { outlinePaint.strokeWidth = 10f } }
        val myLocationOverlay = remember {
            MyLocationNewOverlay(GpsMyLocationProvider(context), mapView)
        }
        val rotationOverlay = remember {
            RotationGestureOverlay(mapView).apply { isEnabled = true }
        }
        val compassOverlay = remember {
            CompassOverlay(context, InternalCompassOrientationProvider(context), mapView)
        }
        val scaleBarOverlay = remember {
            ScaleBarOverlay(mapView).apply { setAlignBottom(true) }
        }

        LaunchedEffect(Unit) {
            mapView.overlays.add(trail)
            mapView.overlays.add(myLocationOverlay)
            mapView.overlays.add(rotationOverlay)
            mapView.overlays.add(compassOverlay)
            mapView.overlays.add(scaleBarOverlay)
            compassOverlay.enableCompass()
        }

        // Default the map theme to the system setting once.
        val systemDark = androidx.compose.foundation.isSystemInDarkTheme()
        LaunchedEffect(systemDark) {
            if (!initializedTheme) {
                darkMap = systemDark
                initializedTheme = true
            }
        }

        // Apply dark/light tiles.
        LaunchedEffect(darkMap) {
            mapView.overlayManager.tilesOverlay.setColorFilter(
                if (darkMap) TilesOverlay.INVERT_COLORS else null
            )
            mapView.invalidate()
        }

        // Enable the blue-dot + auto-snap once location permission is present.
        LaunchedEffect(fineGranted) {
            if (fineGranted) {
                myLocationOverlay.enableMyLocation()
                myLocationOverlay.enableFollowLocation()
                myLocationOverlay.runOnFirstFix {
                    mapView.post {
                        myLocationOverlay.myLocation?.let {
                            mapView.controller.setZoom(17.0)
                            mapView.controller.animateTo(it)
                        }
                    }
                }
            }
        }

        // Live tracked fixes + map lifecycle + resume re-checks.
        DisposableEffect(lifecycleOwner) {
            val receiver = object : BroadcastReceiver() {
                override fun onReceive(c: Context, intent: Intent) {
                    val lat = intent.getDoubleExtra(LocationTrackingService.EXTRA_LAT, Double.NaN)
                    val lon = intent.getDoubleExtra(LocationTrackingService.EXTRA_LON, Double.NaN)
                    if (lat.isNaN() || lon.isNaN()) return
                    trail.addPoint(GeoPoint(lat, lon))
                    mapView.invalidate()
                    tracking = true
                    statusText = "Tracking — %.5f, %.5f".format(lat, lon)
                }
            }
            val filter = IntentFilter(LocationTrackingService.ACTION_LOCATION_UPDATE)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                registerReceiver(receiver, filter, Context.RECEIVER_NOT_EXPORTED)
            } else {
                @Suppress("UnspecifiedRegisterReceiverFlag")
                registerReceiver(receiver, filter)
            }

            val observer = LifecycleEventObserver { _, event ->
                when (event) {
                    Lifecycle.Event.ON_RESUME -> {
                        mapView.onResume()
                        tracking = LocationTrackingService.isRunning
                        fineGranted = hasFineLocation()
                        if (fineGranted) permanentlyDenied = false
                    }
                    Lifecycle.Event.ON_PAUSE -> mapView.onPause()
                    else -> {}
                }
            }
            lifecycleOwner.lifecycle.addObserver(observer)
            onDispose {
                runCatching { unregisterReceiver(receiver) }
                lifecycleOwner.lifecycle.removeObserver(observer)
            }
        }

        // --- permission launchers ---
        val bgLauncher = rememberLauncherForActivityResult(
            ActivityResultContracts.RequestPermission()
        ) { /* background location best-effort */ }

        val requestExtras: () -> Unit = {
            if (needsBackgroundLocation()) {
                bgLauncher.launch(Manifest.permission.ACCESS_BACKGROUND_LOCATION)
            }
            if (!isIgnoringBatteryOptimizations()) {
                promptDisableBatteryOptimization()
            }
        }

        val notifLauncher = rememberLauncherForActivityResult(
            ActivityResultContracts.RequestPermission()
        ) { requestExtras() }

        val afterForegroundGranted: () -> Unit = {
            if (needsNotifPermission()) {
                notifLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
            } else {
                requestExtras()
            }
        }

        val fgLauncher = rememberLauncherForActivityResult(
            ActivityResultContracts.RequestMultiplePermissions()
        ) { grants ->
            fineGranted = grants[Manifest.permission.ACCESS_FINE_LOCATION] == true ||
                grants[Manifest.permission.ACCESS_COARSE_LOCATION] == true
            if (fineGranted) {
                afterForegroundGranted()
            } else {
                // Denied and the system won't show the dialog again -> route to settings.
                permanentlyDenied =
                    !shouldShowRequestPermissionRationale(Manifest.permission.ACCESS_FINE_LOCATION)
            }
        }

        val requestForeground: () -> Unit = {
            fgLauncher.launch(
                arrayOf(
                    Manifest.permission.ACCESS_FINE_LOCATION,
                    Manifest.permission.ACCESS_COARSE_LOCATION
                )
            )
        }

        // First-run enforcement: as soon as the screen appears without location,
        // prompt for it. The scrim (below) keeps the app blocked until granted.
        LaunchedEffect(Unit) {
            if (!fineGranted) requestForeground() else afterForegroundGranted()
        }

        // --- tracking control ---
        val startTracking: () -> Unit = {
            when {
                !isLocationEnabled() -> {
                    statusText = "Turn on device location, then Start again"
                    promptEnableLocation()
                }
                !fineGranted -> requestForeground()
                else -> {
                    startTrackingService()
                    tracking = true
                    statusText = "Tracking — waiting for first fix…"
                    requestExtras()
                }
            }
        }
        val stopTracking: () -> Unit = {
            stopTrackingService()
            tracking = false
            statusText = "Ready to track"
        }

        val snapToLocation: () -> Unit = {
            if (fineGranted) {
                myLocationOverlay.enableFollowLocation()
                myLocationOverlay.myLocation?.let {
                    mapView.controller.setZoom(17.0)
                    mapView.controller.animateTo(it)
                }
            } else {
                requestForeground()
            }
        }

        Box(Modifier.fillMaxSize()) {
            AndroidView(factory = { mapView }, modifier = Modifier.fillMaxSize())

            // Top-right: map light/dark toggle.
            Column(
                modifier = Modifier
                    .align(Alignment.TopEnd)
                    .windowInsetsPadding(WindowInsets.statusBars)
                    .padding(16.dp)
            ) {
                SmallFloatingActionButton(onClick = { darkMap = !darkMap }) {
                    Icon(
                        if (darkMap) Icons.Filled.LightMode else Icons.Filled.DarkMode,
                        contentDescription = "Toggle map theme"
                    )
                }
            }

            // Right side, above the status card: zoom + my-location.
            Column(
                modifier = Modifier
                    .align(Alignment.BottomEnd)
                    .windowInsetsPadding(WindowInsets.navigationBars)
                    .padding(end = 16.dp, bottom = 170.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                SmallFloatingActionButton(onClick = { mapView.controller.zoomIn() }) {
                    Icon(Icons.Filled.Add, contentDescription = "Zoom in")
                }
                SmallFloatingActionButton(onClick = { mapView.controller.zoomOut() }) {
                    Icon(Icons.Filled.Remove, contentDescription = "Zoom out")
                }
                FloatingActionButton(onClick = snapToLocation) {
                    Icon(Icons.Filled.MyLocation, contentDescription = "My location")
                }
            }

            // Floating status / start-stop card.
            Surface(
                modifier = Modifier
                    .align(Alignment.BottomCenter)
                    .windowInsetsPadding(WindowInsets.navigationBars)
                    .padding(16.dp)
                    .fillMaxWidth(),
                shape = RoundedCornerShape(28.dp),
                tonalElevation = 3.dp,
                shadowElevation = 8.dp,
                color = MaterialTheme.colorScheme.surface
            ) {
                Column(Modifier.padding(20.dp)) {
                    Text(statusText, style = MaterialTheme.typography.titleMedium)
                    Spacer(Modifier.height(14.dp))
                    Button(
                        onClick = { if (tracking) stopTracking() else startTracking() },
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(20.dp)
                    ) {
                        Icon(
                            if (tracking) Icons.Filled.LocationOff else Icons.Filled.LocationOn,
                            contentDescription = null
                        )
                        Spacer(Modifier.width(8.dp))
                        Text(if (tracking) "Stop tracking" else "Start tracking")
                    }
                }
            }

            // Enrollment overlay — covers everything until the device is enrolled.
            if (!deviceReady) {
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .background(Color.Black.copy(alpha = 0.75f)),
                    contentAlignment = Alignment.Center
                ) {
                    Surface(
                        modifier = Modifier.padding(28.dp).fillMaxWidth(),
                        shape = RoundedCornerShape(28.dp),
                        tonalElevation = 6.dp,
                        shadowElevation = 12.dp
                    ) {
                        Column(Modifier.padding(24.dp)) {
                            Text(
                                if (enrollError == null) "Setting up your device…" else "Setup failed",
                                style = MaterialTheme.typography.headlineSmall
                            )
                            Spacer(Modifier.height(12.dp))
                            Text(
                                enrollError ?: "Registering with the server. This device will appear in the admin dashboard as pending until it is assigned to an organization.",
                                style = MaterialTheme.typography.bodyMedium
                            )
                            if (enrollError != null) {
                                Spacer(Modifier.height(20.dp))
                                Button(
                                    onClick = { enrollAttempt++ },
                                    modifier = Modifier.fillMaxWidth(),
                                    shape = RoundedCornerShape(20.dp)
                                ) { Text("Retry") }
                            }
                        }
                    }
                }
            }

            // Blocking permission scrim (first-run enforcement).
            if (!fineGranted) {
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .background(Color.Black.copy(alpha = 0.6f)),
                    contentAlignment = Alignment.Center
                ) {
                    Surface(
                        modifier = Modifier
                            .padding(28.dp)
                            .fillMaxWidth(),
                        shape = RoundedCornerShape(28.dp),
                        tonalElevation = 6.dp,
                        shadowElevation = 12.dp
                    ) {
                        Column(Modifier.padding(24.dp)) {
                            Text(
                                "Location access needed",
                                style = MaterialTheme.typography.headlineSmall
                            )
                            Spacer(Modifier.height(12.dp))
                            Text(
                                if (permanentlyDenied)
                                    "Location permission is turned off. Open settings and allow location to use TrackProj."
                                else
                                    "TrackProj needs location access to track this device. Please allow it to continue.",
                                style = MaterialTheme.typography.bodyMedium,
                                textAlign = TextAlign.Start
                            )
                            Spacer(Modifier.height(20.dp))
                            Button(
                                onClick = {
                                    if (permanentlyDenied) openAppSettings() else requestForeground()
                                },
                                modifier = Modifier.fillMaxWidth(),
                                shape = RoundedCornerShape(20.dp)
                            ) {
                                Text(if (permanentlyDenied) "Open settings" else "Grant permission")
                            }
                        }
                    }
                }
            }
        }
    }
}
