package com.trackproj.app.tracking

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import android.util.Log
import androidx.core.app.NotificationCompat
import com.google.android.gms.location.FusedLocationProviderClient
import com.google.android.gms.location.LocationCallback
import com.google.android.gms.location.LocationRequest
import com.google.android.gms.location.LocationResult
import com.google.android.gms.location.LocationServices
import com.google.android.gms.location.Priority
import com.trackproj.app.R
import com.trackproj.app.auth.ApiClient
import com.trackproj.app.auth.TokenStore
import java.time.Instant
import java.util.concurrent.Executors

/**
 * Foreground service that captures location periodically via
 * FusedLocationProviderClient and uploads each fix to POST /v1/positions
 * using the stored device token. Broadcasts each accepted fix locally so the
 * MainActivity map can render the live trail.
 */
class LocationTrackingService : Service() {

    private lateinit var fused: FusedLocationProviderClient
    private lateinit var tokenStore: TokenStore
    private val api = ApiClient()
    // Serialize uploads so fixes post in order and never block the main thread.
    private val uploadExecutor = Executors.newSingleThreadExecutor()

    private val locationCallback = object : LocationCallback() {
        override fun onLocationResult(result: LocationResult) {
            val loc = result.lastLocation ?: return
            val token = tokenStore.deviceToken
            if (token == null) {
                Log.w(TAG, "no device token stored; skipping upload")
                return
            }
            val speed = if (loc.hasSpeed()) loc.speed.toDouble() else null
            val heading = if (loc.hasBearing()) loc.bearing.toDouble() else null
            val accuracy = if (loc.hasAccuracy()) loc.accuracy.toDouble() else null

            uploadExecutor.execute {
                try {
                    api.postPosition(
                        deviceToken = token,
                        lat = loc.latitude,
                        lon = loc.longitude,
                        speed = speed,
                        heading = heading,
                        accuracy = accuracy,
                        recordedAt = Instant.ofEpochMilli(loc.time)
                    )
                    Log.d(TAG, "posted ${loc.latitude},${loc.longitude}")
                } catch (e: ApiClient.DeviceUnauthorizedException) {
                    // Backend forgot this device (deleted/revoked). Re-enroll silently
                    // so it reappears in the dashboard as pending, then repost this fix.
                    Log.w(TAG, "token rejected; re-enrolling: ${e.message}")
                    try {
                        val d = api.enroll("${Build.MANUFACTURER} ${Build.MODEL}")
                        tokenStore.deviceId = d.id
                        tokenStore.deviceToken = d.apiToken
                        api.postPosition(
                            deviceToken = d.apiToken,
                            lat = loc.latitude,
                            lon = loc.longitude,
                            speed = speed,
                            heading = heading,
                            accuracy = accuracy,
                            recordedAt = Instant.ofEpochMilli(loc.time)
                        )
                        Log.d(TAG, "re-enrolled as ${d.id}; reposted fix")
                    } catch (re: Exception) {
                        Log.e(TAG, "re-enroll failed: ${re.message}")
                    }
                } catch (e: Exception) {
                    Log.e(TAG, "upload failed: ${e.message}")
                }
            }

            // Notify the UI (in-process broadcast) regardless of upload result.
            sendBroadcast(Intent(ACTION_LOCATION_UPDATE).apply {
                setPackage(packageName)
                putExtra(EXTRA_LAT, loc.latitude)
                putExtra(EXTRA_LON, loc.longitude)
            })
        }
    }

    override fun onCreate() {
        super.onCreate()
        fused = LocationServices.getFusedLocationProviderClient(this)
        tokenStore = TokenStore(this)
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> {
                stopTracking()
                return START_NOT_STICKY
            }
            else -> startTracking()
        }
        return START_STICKY
    }

    @Suppress("MissingPermission") // caller (MainActivity) guarantees the grant
    private fun startTracking() {
        startForegroundWithNotification()

        val request = LocationRequest.Builder(
            Priority.PRIORITY_HIGH_ACCURACY,
            UPDATE_INTERVAL_MS
        )
            .setMinUpdateIntervalMillis(MIN_UPDATE_INTERVAL_MS)
            .build()

        try {
            fused.requestLocationUpdates(request, locationCallback, mainLooper)
            isRunning = true
        } catch (e: SecurityException) {
            Log.e(TAG, "location permission missing: ${e.message}")
            stopSelf()
        }
    }

    private fun stopTracking() {
        fused.removeLocationUpdates(locationCallback)
        isRunning = false
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private fun startForegroundWithNotification() {
        val notification: Notification = NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("TrackProj is tracking")
            .setContentText("Sharing your location with the dashboard")
            .setSmallIcon(R.drawable.ic_launcher_foreground)
            .setOngoing(true)
            .build()

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(
                NOTIFICATION_ID,
                notification,
                ServiceInfo.FOREGROUND_SERVICE_TYPE_LOCATION
            )
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "Location tracking",
                NotificationManager.IMPORTANCE_LOW
            )
            val mgr = getSystemService(NotificationManager::class.java)
            mgr.createNotificationChannel(channel)
        }
    }

    override fun onDestroy() {
        fused.removeLocationUpdates(locationCallback)
        isRunning = false
        uploadExecutor.shutdown()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    companion object {
        private const val TAG = "LocationTracking"
        private const val CHANNEL_ID = "location_tracking"
        private const val NOTIFICATION_ID = 1
        private const val UPDATE_INTERVAL_MS = 15_000L
        private const val MIN_UPDATE_INTERVAL_MS = 10_000L

        /** True while the service is actively tracking (same-process read for the UI). */
        @Volatile
        var isRunning: Boolean = false
            private set

        const val ACTION_START = "com.trackproj.app.action.START_TRACKING"
        const val ACTION_STOP = "com.trackproj.app.action.STOP_TRACKING"
        const val ACTION_LOCATION_UPDATE = "com.trackproj.app.LOCATION_UPDATE"
        const val EXTRA_LAT = "lat"
        const val EXTRA_LON = "lon"
    }
}
