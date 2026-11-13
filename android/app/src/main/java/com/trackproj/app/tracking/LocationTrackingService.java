package com.trackproj.app.tracking;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.Service;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.location.Location;
import android.os.Build;
import android.os.IBinder;
import android.util.Log;

import androidx.annotation.NonNull;
import androidx.core.app.NotificationCompat;

import com.google.android.gms.location.FusedLocationProviderClient;
import com.google.android.gms.location.LocationCallback;
import com.google.android.gms.location.LocationRequest;
import com.google.android.gms.location.LocationResult;
import com.google.android.gms.location.LocationServices;
import com.google.android.gms.location.Priority;
import com.trackproj.app.R;
import com.trackproj.app.auth.ApiClient;
import com.trackproj.app.auth.TokenStore;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * Foreground service that captures location periodically with
 * FusedLocationProviderClient and uploads each fix to POST /v1/positions using
 * the stored device token. It broadcasts every accepted fix locally so
 * MainActivity can draw the live trail.
 *
 * Java port of the original Kotlin LocationTrackingService; behaviour and
 * constants are unchanged.
 */
public class LocationTrackingService extends Service {

    private static final String TAG = "LocationTracking";
    private static final String CHANNEL_ID = "location_tracking";
    private static final int NOTIFICATION_ID = 1;
    private static final long UPDATE_INTERVAL_MS = 15_000L;
    private static final long MIN_UPDATE_INTERVAL_MS = 10_000L;

    public static final String ACTION_START = "com.trackproj.app.action.START_TRACKING";
    public static final String ACTION_STOP = "com.trackproj.app.action.STOP_TRACKING";
    public static final String ACTION_LOCATION_UPDATE = "com.trackproj.app.LOCATION_UPDATE";
    public static final String EXTRA_LAT = "lat";
    public static final String EXTRA_LON = "lon";

    /** True while the service is actively tracking (same-process read for the UI). */
    private static volatile boolean running = false;

    public static boolean isRunning() {
        return running;
    }

    private FusedLocationProviderClient fused;
    private TokenStore tokenStore;

    private final ApiClient api = new ApiClient();
    // Serialize uploads so fixes post in order and never block the main thread.
    private final ExecutorService uploadExecutor = Executors.newSingleThreadExecutor();

    private final LocationCallback locationCallback = new LocationCallback() {
        @Override
        public void onLocationResult(@NonNull LocationResult result) {
            Location location = result.getLastLocation();
            if (location == null) {
                return;
            }

            final String token = tokenStore.getDeviceToken();
            if (token == null) {
                Log.w(TAG, "no device token stored; skipping upload");
                return;
            }

            final Location fix = location;
            final Double speed = fix.hasSpeed() ? (double) fix.getSpeed() : null;
            final Double heading = fix.hasBearing() ? (double) fix.getBearing() : null;
            final Double accuracy = fix.hasAccuracy() ? (double) fix.getAccuracy() : null;

            uploadExecutor.execute(() -> uploadFix(fix, token, speed, heading, accuracy));

            // Notify the UI (in-process broadcast) regardless of upload result.
            Intent update = new Intent(ACTION_LOCATION_UPDATE);
            update.setPackage(getPackageName());
            update.putExtra(EXTRA_LAT, fix.getLatitude());
            update.putExtra(EXTRA_LON, fix.getLongitude());
            sendBroadcast(update);
        }
    };

    private void uploadFix(Location fix, String token,
                           Double speed, Double heading, Double accuracy) {
        try {
            api.postPosition(token, fix.getLatitude(), fix.getLongitude(),
                    speed, heading, accuracy, fix.getTime());
            Log.d(TAG, "posted " + fix.getLatitude() + "," + fix.getLongitude());
        } catch (ApiClient.DeviceUnauthorizedException e) {
            // Backend forgot this device (deleted/revoked). Re-enroll silently so
            // it reappears in the dashboard as pending, then repost this fix.
            Log.w(TAG, "token rejected; re-enrolling: " + e.getMessage());
            try {
                ApiClient.DeviceResult device = api.enroll(
                        Build.MANUFACTURER + " " + Build.MODEL);
                tokenStore.setDeviceId(device.id);
                tokenStore.setDeviceToken(device.apiToken);
                api.postPosition(device.apiToken, fix.getLatitude(), fix.getLongitude(),
                        speed, heading, accuracy, fix.getTime());
                Log.d(TAG, "re-enrolled as " + device.id + "; reposted fix");
            } catch (Exception re) {
                Log.e(TAG, "re-enroll failed: " + re.getMessage());
            }
        } catch (Exception e) {
            Log.e(TAG, "upload failed: " + e.getMessage());
        }
    }

    @Override
    public void onCreate() {
        super.onCreate();
        fused = LocationServices.getFusedLocationProviderClient(this);
        tokenStore = new TokenStore(this);
        createNotificationChannel();
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        if (intent != null && ACTION_STOP.equals(intent.getAction())) {
            stopTracking();
            return START_NOT_STICKY;
        }
        startTracking();
        return START_STICKY;
    }

    private void startTracking() {
        startForegroundWithNotification();

        LocationRequest request = new LocationRequest.Builder(
                Priority.PRIORITY_HIGH_ACCURACY, UPDATE_INTERVAL_MS)
                .setMinUpdateIntervalMillis(MIN_UPDATE_INTERVAL_MS)
                .build();

        try {
            fused.requestLocationUpdates(request, locationCallback, getMainLooper());
            running = true;
            reportTrackingState(true);
        } catch (SecurityException e) {
            Log.e(TAG, "location permission missing: " + e.getMessage());
            stopSelf();
        }
    }

    private void stopTracking() {
        fused.removeLocationUpdates(locationCallback);
        running = false;
        reportTrackingState(false);
        stopForeground(STOP_FOREGROUND_REMOVE);
        stopSelf();
    }

    private void reportTrackingState(boolean active) {
        String token = tokenStore.getDeviceToken();
        if (token == null) {
            return;
        }
        uploadExecutor.execute(() -> {
            try {
                api.setTrackingState(token, active);
            } catch (Exception e) {
                Log.w(TAG, "tracking-state report failed: " + e.getMessage());
            }
        });
    }

    private void startForegroundWithNotification() {
        Notification notification = new NotificationCompat.Builder(this, CHANNEL_ID)
                .setContentTitle("TrackStack is tracking")
                .setContentText("Sharing your location with the dashboard")
                .setSmallIcon(R.drawable.ic_stat_track)
                .setOngoing(true)
                .build();

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(NOTIFICATION_ID, notification,
                    ServiceInfo.FOREGROUND_SERVICE_TYPE_LOCATION);
        } else {
            startForeground(NOTIFICATION_ID, notification);
        }
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                    CHANNEL_ID, "Location tracking", NotificationManager.IMPORTANCE_LOW);
            NotificationManager manager = getSystemService(NotificationManager.class);
            if (manager != null) {
                manager.createNotificationChannel(channel);
            }
        }
    }

    @Override
    public void onDestroy() {
        fused.removeLocationUpdates(locationCallback);
        running = false;
        uploadExecutor.shutdown();
        super.onDestroy();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}
