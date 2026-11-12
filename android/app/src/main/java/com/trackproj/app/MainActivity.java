package com.trackproj.app;

import android.Manifest;
import android.annotation.SuppressLint;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.pm.PackageManager;
import android.location.LocationManager;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.PowerManager;
import android.provider.Settings;
import android.view.View;
import android.widget.TextView;

import androidx.activity.result.ActivityResultLauncher;
import androidx.activity.result.contract.ActivityResultContracts;
import androidx.annotation.NonNull;
import androidx.appcompat.app.AppCompatActivity;
import androidx.core.content.ContextCompat;

import com.google.android.material.button.MaterialButton;
import com.google.android.material.floatingactionbutton.FloatingActionButton;
import com.trackproj.app.auth.ApiClient;
import com.trackproj.app.auth.TokenStore;
import com.trackproj.app.tracking.LocationTrackingService;

import org.osmdroid.config.Configuration;
import org.osmdroid.tileprovider.tilesource.TileSourceFactory;
import org.osmdroid.util.GeoPoint;
import org.osmdroid.views.CustomZoomButtonsController;
import org.osmdroid.views.MapView;
import org.osmdroid.views.overlay.Polyline;
import org.osmdroid.views.overlay.ScaleBarOverlay;
import org.osmdroid.views.overlay.TilesOverlay;
import org.osmdroid.views.overlay.compass.CompassOverlay;
import org.osmdroid.views.overlay.compass.InternalCompassOrientationProvider;
import org.osmdroid.views.overlay.gestures.RotationGestureOverlay;
import org.osmdroid.views.overlay.mylocation.GpsMyLocationProvider;
import org.osmdroid.views.overlay.mylocation.MyLocationNewOverlay;

import java.util.Locale;
import java.util.Map;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * Single-screen tracker.
 *
 * <p>Java/XML port of the original Jetpack Compose screen. Responsibilities are
 * unchanged: silently enroll on first launch, host the osmdroid map with the
 * trail / my-location / rotation / compass / scale overlays, gate the UI behind
 * the location permission, and drive the foreground tracking service.
 *
 * <p>Layout: {@code res/layout/activity_main.xml}.
 */
public class MainActivity extends AppCompatActivity {

    private static final String STATE_DARK_MAP = "dark_map";
    private static final String STATE_TRACKING = "tracking";

    private TokenStore tokenStore;

    // --- views ---
    private MapView map;
    private TextView statusText;
    private MaterialButton btnTracking;
    private FloatingActionButton btnTheme;
    private View enrollOverlay;
    private TextView enrollTitle;
    private TextView enrollBody;
    private MaterialButton btnRetry;
    private View permissionOverlay;
    private TextView permissionBody;
    private MaterialButton btnPermission;

    // --- map overlays ---
    private Polyline trail;
    private MyLocationNewOverlay myLocationOverlay;
    private CompassOverlay compassOverlay;
    private ScaleBarOverlay scaleBarOverlay;

    // --- state ---
    private boolean tracking;
    /** A start was requested but the service has not confirmed it is running yet. */
    private boolean startRequested;
    private boolean darkMap;
    private boolean deviceReady;
    private boolean fineGranted;
    private boolean permanentlyDenied;

    /** Background work for enrollment (network I/O must never touch the UI thread). */
    private final ExecutorService io = Executors.newSingleThreadExecutor();

    private boolean receiverRegistered;

    private final BroadcastReceiver locationReceiver = new BroadcastReceiver() {
        @Override
        public void onReceive(Context context, Intent intent) {
            double lat = intent.getDoubleExtra(LocationTrackingService.EXTRA_LAT, Double.NaN);
            double lon = intent.getDoubleExtra(LocationTrackingService.EXTRA_LON, Double.NaN);
            if (Double.isNaN(lat) || Double.isNaN(lon)) {
                return;
            }
            if (trail != null && map != null) {
                trail.addPoint(new GeoPoint(lat, lon));
                map.invalidate();
            }
            tracking = true;
            renderTrackingState();
            statusText.setText(String.format(Locale.US,
                    "Tracking \u2014 %.5f, %.5f", lat, lon));
        }
    };

    // --- permission launchers (must be registered before the activity resumes) ---

    /** POST_NOTIFICATIONS result: continue the chain toward background location. */
    private final ActivityResultLauncher<String> notifLauncher =
            registerForActivityResult(new ActivityResultContracts.RequestPermission(),
                    granted -> requestBackgroundExtras());

    /** ACCESS_BACKGROUND_LOCATION result: best effort; then battery optimization. */
    private final ActivityResultLauncher<String> backgroundLauncher =
            registerForActivityResult(new ActivityResultContracts.RequestPermission(),
                    granted -> maybePromptBatteryOptimization());

    /** FINE + COARSE result: unlocks the UI or routes the user to system settings. */
    private final ActivityResultLauncher<String[]> foregroundLauncher =
            registerForActivityResult(new ActivityResultContracts.RequestMultiplePermissions(),
                    this::onForegroundPermissionResult);

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        Configuration.getInstance().setUserAgentValue(getPackageName());

        setContentView(R.layout.activity_main);
        tokenStore = new TokenStore(this);

        bindViews();
        restoreState(savedInstanceState);
        setUpMap();
        renderTrackingState();
        statusText.setText(tracking ? R.string.tracking_on : R.string.ready_to_track);

        fineGranted = hasFineLocation();
        renderPermissionState();
        if (fineGranted) {
            enableLocationOverlay();
        }

        deviceReady = tokenStore.hasDevice();
        enrollOverlay.setVisibility(deviceReady ? View.GONE : View.VISIBLE);

        btnTheme.setOnClickListener(v -> {
            darkMap = !darkMap;
            applyMapTheme();
        });
        btnTracking.setOnClickListener(v -> {
            if (tracking) {
                stopTracking();
            } else {
                startTracking();
            }
        });
        btnRetry.setOnClickListener(v -> startEnrollment());
        btnPermission.setOnClickListener(v -> {
            if (permanentlyDenied) {
                openAppSettings();
            } else {
                requestForegroundPermissions();
            }
        });

        if (!deviceReady) {
            startEnrollment();
        }

        // First-run enforcement: as soon as the screen appears without location,
        // prompt for it. The scrim keeps the app blocked until granted.
        final View root = findViewById(R.id.root);
        root.post(() -> {
            if (isFinishing() || isDestroyed()) {
                return;
            }
            if (!fineGranted) {
                requestForegroundPermissions();
            } else {
                afterForegroundGranted();
            }
        });
    }

    private void bindViews() {
        map = findViewById(R.id.map);
        statusText = findViewById(R.id.statusText);
        btnTracking = findViewById(R.id.btnTracking);
        btnTheme = findViewById(R.id.btnTheme);
        enrollOverlay = findViewById(R.id.enrollOverlay);
        enrollTitle = findViewById(R.id.enrollTitle);
        enrollBody = findViewById(R.id.enrollBody);
        btnRetry = findViewById(R.id.btnRetry);
        permissionOverlay = findViewById(R.id.permissionOverlay);
        permissionBody = findViewById(R.id.permissionBody);
        btnPermission = findViewById(R.id.btnPermission);

        findViewById(R.id.btnZoomIn).setOnClickListener(v -> map.getController().zoomIn());
        findViewById(R.id.btnZoomOut).setOnClickListener(v -> map.getController().zoomOut());
        findViewById(R.id.btnMyLocation).setOnClickListener(v -> snapToLocation());
    }

    // ------------------------------------------------------------------
    // Map
    // ------------------------------------------------------------------

    private void setUpMap() {
        map.setTileSource(TileSourceFactory.MAPNIK);
        map.setMultiTouchControls(true);
        map.getZoomController().setVisibility(CustomZoomButtonsController.Visibility.NEVER);
        map.getController().setZoom(16.0);

        trail = new Polyline();
        trail.getOutlinePaint().setStrokeWidth(10f);
        trail.getOutlinePaint().setColor(ContextCompat.getColor(this, R.color.primary));

        myLocationOverlay = new MyLocationNewOverlay(new GpsMyLocationProvider(this), map);
        RotationGestureOverlay rotationOverlay = new RotationGestureOverlay(map);
        rotationOverlay.setEnabled(true);
        compassOverlay = new CompassOverlay(this,
                new InternalCompassOrientationProvider(this), map);
        scaleBarOverlay = new ScaleBarOverlay(map);
        scaleBarOverlay.setAlignBottom(true);

        map.getOverlays().add(trail);
        map.getOverlays().add(myLocationOverlay);
        map.getOverlays().add(rotationOverlay);
        map.getOverlays().add(compassOverlay);
        map.getOverlays().add(scaleBarOverlay);

        compassOverlay.enableCompass();
        applyMapTheme();
    }

    /** Dark mode inverts the raster tiles; the default follows the system setting. */
    private void applyMapTheme() {
        TilesOverlay tiles = map.getOverlayManager().getTilesOverlay();
        if (tiles != null) {
            tiles.setColorFilter(darkMap ? TilesOverlay.INVERT_COLORS : null);
        }
        btnTheme.setImageResource(darkMap
                ? R.drawable.ic_light_mode
                : R.drawable.ic_dark_mode);
        map.invalidate();
    }

    private boolean isSystemDark() {
        int mode = getResources().getConfiguration().uiMode
                & android.content.res.Configuration.UI_MODE_NIGHT_MASK;
        return mode == android.content.res.Configuration.UI_MODE_NIGHT_YES;
    }

    @SuppressLint("MissingPermission")
    private void enableLocationOverlay() {
        if (!fineGranted || myLocationOverlay == null) {
            return;
        }
        try {
            myLocationOverlay.enableMyLocation();
            myLocationOverlay.enableFollowLocation();
            myLocationOverlay.runOnFirstFix(() -> runOnUiThread(() -> {
                GeoPoint fix = myLocationOverlay.getMyLocation();
                if (fix != null && map != null) {
                    map.getController().setZoom(17.0);
                    map.getController().animateTo(fix);
                }
            }));
        } catch (SecurityException ignored) {
            // Permission revoked between check and call.
        }
    }

    private void snapToLocation() {
        if (!fineGranted) {
            requestForegroundPermissions();
            return;
        }
        myLocationOverlay.enableFollowLocation();
        GeoPoint fix = myLocationOverlay.getMyLocation();
        if (fix != null) {
            map.getController().setZoom(17.0);
            map.getController().animateTo(fix);
        }
    }

    // ------------------------------------------------------------------
    // Enrollment
    // ------------------------------------------------------------------

    private void startEnrollment() {
        enrollTitle.setText(R.string.setting_up);
        enrollBody.setText(R.string.enroll_body);
        btnRetry.setVisibility(View.GONE);
        enrollOverlay.setVisibility(View.VISIBLE);

        io.execute(() -> {
            try {
                if (!tokenStore.hasDevice()) {
                    ApiClient.DeviceResult device =
                            new ApiClient().enroll(Build.MANUFACTURER + " " + Build.MODEL);
                    tokenStore.setDeviceId(device.id);
                    tokenStore.setDeviceToken(device.apiToken);
                }
                runOnUiThread(() -> {
                    if (isFinishing() || isDestroyed()) {
                        return;
                    }
                    deviceReady = true;
                    enrollOverlay.setVisibility(View.GONE);
                });
            } catch (Exception e) {
                final String message = e.getMessage() != null
                        ? e.getMessage()
                        : "enrollment failed";
                runOnUiThread(() -> {
                    if (isFinishing() || isDestroyed()) {
                        return;
                    }
                    enrollTitle.setText(R.string.setup_failed);
                    enrollBody.setText(message);
                    btnRetry.setVisibility(View.VISIBLE);
                });
            }
        });
    }

    // ------------------------------------------------------------------
    // Permissions
    // ------------------------------------------------------------------

    private boolean hasFineLocation() {
        return ContextCompat.checkSelfPermission(this,
                Manifest.permission.ACCESS_FINE_LOCATION) == PackageManager.PERMISSION_GRANTED;
    }

    private boolean needsNotifPermission() {
        return Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU
                && ContextCompat.checkSelfPermission(this,
                Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED;
    }

    private boolean needsBackgroundLocation() {
        return Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q
                && ContextCompat.checkSelfPermission(this,
                Manifest.permission.ACCESS_BACKGROUND_LOCATION)
                != PackageManager.PERMISSION_GRANTED;
    }

    private void requestForegroundPermissions() {
        foregroundLauncher.launch(new String[]{
                Manifest.permission.ACCESS_FINE_LOCATION,
                Manifest.permission.ACCESS_COARSE_LOCATION
        });
    }

    private void onForegroundPermissionResult(@NonNull Map<String, Boolean> grants) {
        fineGranted = Boolean.TRUE.equals(grants.get(Manifest.permission.ACCESS_FINE_LOCATION))
                || Boolean.TRUE.equals(grants.get(Manifest.permission.ACCESS_COARSE_LOCATION));
        renderPermissionState();
        if (fineGranted) {
            permanentlyDenied = false;
            enableLocationOverlay();
            afterForegroundGranted();
        } else {
            // Denied and the system won't show the dialog again -> route to settings.
            permanentlyDenied = !shouldShowRequestPermissionRationale(
                    Manifest.permission.ACCESS_FINE_LOCATION);
            renderPermissionState();
        }
    }

    private void afterForegroundGranted() {
        if (needsNotifPermission()) {
            notifLauncher.launch(Manifest.permission.POST_NOTIFICATIONS);
        } else {
            requestBackgroundExtras();
        }
    }

    /** Background location (API 29+) then battery-optimization exemption. */
    private void requestBackgroundExtras() {
        if (needsBackgroundLocation()) {
            backgroundLauncher.launch(Manifest.permission.ACCESS_BACKGROUND_LOCATION);
        } else {
            maybePromptBatteryOptimization();
        }
    }

    private boolean isIgnoringBatteryOptimizations() {
        PowerManager pm = getSystemService(PowerManager.class);
        return pm != null && pm.isIgnoringBatteryOptimizations(getPackageName());
    }

    @SuppressLint("BatteryLife")
    private void maybePromptBatteryOptimization() {
        if (isIgnoringBatteryOptimizations()) {
            return;
        }
        try {
            startActivity(new Intent(
                    Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS,
                    Uri.parse("package:" + getPackageName())));
        } catch (Exception ignored) {
            // Some OEM builds have no handler; not fatal.
        }
    }

    private void openAppSettings() {
        startActivity(new Intent(
                Settings.ACTION_APPLICATION_DETAILS_SETTINGS,
                Uri.parse("package:" + getPackageName())));
    }

    private void renderPermissionState() {
        permissionOverlay.setVisibility(fineGranted ? View.GONE : View.VISIBLE);
        permissionBody.setText(permanentlyDenied
                ? R.string.permission_off_body
                : R.string.permission_body);
        btnPermission.setText(permanentlyDenied
                ? R.string.open_settings
                : R.string.grant_permission);
    }

    // ------------------------------------------------------------------
    // Tracking control
    // ------------------------------------------------------------------

    private boolean isLocationEnabled() {
        LocationManager lm = getSystemService(LocationManager.class);
        if (lm == null) {
            return false;
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            return lm.isLocationEnabled();
        }
        return lm.isProviderEnabled(LocationManager.GPS_PROVIDER)
                || lm.isProviderEnabled(LocationManager.NETWORK_PROVIDER);
    }

    private void startTracking() {
        if (!isLocationEnabled()) {
            statusText.setText(R.string.turn_on_location);
            startActivity(new Intent(Settings.ACTION_LOCATION_SOURCE_SETTINGS));
            return;
        }
        if (!fineGranted) {
            requestForegroundPermissions();
            return;
        }

        Intent i = new Intent(this, LocationTrackingService.class);
        i.setAction(LocationTrackingService.ACTION_START);
        ContextCompat.startForegroundService(this, i);

        startRequested = true;
        tracking = true;
        renderTrackingState();
        statusText.setText(R.string.tracking_waiting);
        requestBackgroundExtras();
    }

    private void stopTracking() {
        Intent i = new Intent(this, LocationTrackingService.class);
        i.setAction(LocationTrackingService.ACTION_STOP);
        startService(i);

        startRequested = false;
        tracking = false;
        renderTrackingState();
        statusText.setText(R.string.ready_to_track);
    }

    private void renderTrackingState() {
        if (btnTracking == null) {
            return;
        }
        btnTracking.setText(tracking ? R.string.stop_tracking : R.string.start_tracking);
        btnTracking.setIconResource(tracking
                ? R.drawable.ic_stop
                : R.drawable.ic_location_on);
    }

    // ------------------------------------------------------------------
    // Lifecycle
    // ------------------------------------------------------------------

    @Override
    protected void onResume() {
        super.onResume();
        if (map != null) {
            map.onResume();
        }
        if (compassOverlay != null) {
            compassOverlay.enableCompass();
        }

        boolean wasGranted = fineGranted;
        fineGranted = hasFineLocation();
        if (fineGranted) {
            permanentlyDenied = false;
        }
        renderPermissionState();
        if (fineGranted && !wasGranted) {
            enableLocationOverlay();
        }

        tracking = LocationTrackingService.isRunning();
        if (tracking) {
            startRequested = false;
        } else if (startRequested) {
            // Start is in flight; don't flip the button back under the user.
            tracking = true;
        } else {
            tracking = false;
        }
        renderTrackingState();
        if (tracking) {
            if (statusText.getText().length() == 0
                    || statusText.getText().toString()
                    .equals(getString(R.string.ready_to_track))) {
                statusText.setText(R.string.tracking_on);
            }
        } else if (statusText.getText().toString()
                .equals(getString(R.string.tracking_on))) {
            statusText.setText(R.string.ready_to_track);
        }

        registerLocationReceiver();
    }

    @Override
    protected void onPause() {
        if (map != null) {
            map.onPause();
        }
        if (receiverRegistered) {
            try {
                unregisterReceiver(locationReceiver);
            } catch (IllegalArgumentException ignored) {
                // Already unregistered.
            }
            receiverRegistered = false;
        }
        super.onPause();
    }

    private void registerLocationReceiver() {
        if (receiverRegistered) {
            return;
        }
        IntentFilter filter = new IntentFilter(LocationTrackingService.ACTION_LOCATION_UPDATE);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            registerReceiver(locationReceiver, filter, Context.RECEIVER_NOT_EXPORTED);
        } else {
            registerReceiver(locationReceiver, filter);
        }
        receiverRegistered = true;
    }

    @Override
    protected void onSaveInstanceState(@NonNull Bundle outState) {
        super.onSaveInstanceState(outState);
        outState.putBoolean(STATE_DARK_MAP, darkMap);
        outState.putBoolean(STATE_TRACKING, tracking);
    }

    private void restoreState(Bundle savedInstanceState) {
        if (savedInstanceState != null) {
            darkMap = savedInstanceState.getBoolean(STATE_DARK_MAP, isSystemDark());
            tracking = savedInstanceState.getBoolean(STATE_TRACKING, false);
        } else {
            darkMap = isSystemDark();
        }
        tracking = tracking || LocationTrackingService.isRunning();
        // The button label starts from the service state, otherwise "Ready to track".
    }

    @Override
    protected void onDestroy() {
        io.shutdown();
        if (map != null) {
            map.onDetach();
        }
        super.onDestroy();
    }
}
