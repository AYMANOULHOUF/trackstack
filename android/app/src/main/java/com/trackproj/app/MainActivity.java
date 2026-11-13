package com.trackproj.app;

import android.Manifest;
import android.annotation.SuppressLint;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.pm.PackageManager;
import android.content.res.ColorStateList;
import android.graphics.drawable.Drawable;
import android.location.LocationManager;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.os.PowerManager;
import android.provider.Settings;
import android.view.LayoutInflater;
import android.view.View;
import android.widget.ImageButton;
import android.widget.LinearLayout;
import android.widget.TextView;
import android.widget.Toast;

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
import com.trackproj.app.ui.DonutView;
import com.trackproj.app.ui.SparklineView;

import org.osmdroid.config.Configuration;
import org.osmdroid.tileprovider.tilesource.TileSourceFactory;
import org.osmdroid.util.BoundingBox;
import org.osmdroid.util.GeoPoint;
import org.osmdroid.views.CustomZoomButtonsController;
import org.osmdroid.views.MapView;
import org.osmdroid.views.overlay.Marker;
import org.osmdroid.views.overlay.Polyline;
import org.osmdroid.views.overlay.TilesOverlay;

import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Deque;
import java.util.Locale;
import java.util.Map;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * Device dashboard.
 *
 * <p>Java/XML port of the original Compose tracker, rebuilt as a telemetry
 * dashboard: status hero, session metrics, speed sparkline, live trail map,
 * time-distribution ring and a local activity feed. The device token can only
 * post fixes (it cannot read the org API), so every number here is computed
 * from the fixes this phone produced while the screen was open.
 *
 * <p>Enrollment and the permission chain are unchanged.
 */
public class MainActivity extends AppCompatActivity {

    private static final String STATE_DARK_MAP = "dark_map";
    private static final String STATE_TRACKING = "tracking";
    private static final String STATE_DISTANCE = "distance_m";
    private static final String STATE_FIXES = "fixes";
    private static final String STATE_MOVING = "moving_s";
    private static final String STATE_IDLE = "idle_s";
    private static final String STATE_SILENT = "silent_s";
    private static final String STATE_LAST_LAT = "last_lat";
    private static final String STATE_LAST_LON = "last_lon";
    private static final String STATE_LAST_SPEED = "last_speed";
    private static final String STATE_LAST_ACCURACY = "last_accuracy";
    private static final String STATE_LAST_FIX_AT = "last_fix_at";
    private static final String STATE_SESSION_START = "session_start";
    private static final String STATE_TRAIL_LAT = "trail_lat";
    private static final String STATE_TRAIL_LON = "trail_lon";
    private static final String STATE_SPEEDS = "speeds";

    /** A gap larger than this counts as lost signal in the ring + feed. */
    private static final long SIGNAL_GAP_SECONDS = 90;
    private static final int MAX_TRAIL_POINTS = 600;
    private static final int MAX_CHART_SAMPLES = 80;
    private static final int MAX_EVENTS = 6;

    private TokenStore tokenStore;

    // --- dashboard views ---
    private MapView map;
    private TextView deviceName;
    private TextView chipLive;
    private TextView statusText;
    private TextView lastFixText;
    private TextView coordsText;
    private TextView accuracyChip;
    private TextView accuracyText;
    private TextView speedValue;
    private TextView distanceValue;
    private TextView fixesValue;
    private TextView chartPeak;
    private TextView chartEmpty;
    private TextView mapChip;
    private TextView mapEmpty;
    private TextView donutTotal;
    private TextView donutUnit;
    private TextView legendMovingPct;
    private TextView legendIdlePct;
    private TextView legendSilentPct;
    private TextView legendMovingDur;
    private TextView legendIdleDur;
    private TextView legendSilentDur;
    private TextView activityCount;
    private TextView activityEmpty;
    private LinearLayout activityList;
    private SparklineView speedChart;
    private DonutView donut;
    private MaterialButton btnTracking;
    private ImageButton btnTheme;
    private FloatingActionButton btnMyLocation;
    private View statusDot;

    private View enrollOverlay;
    private TextView enrollTitle;
    private TextView enrollBody;
    private MaterialButton btnRetry;
    private View permissionOverlay;
    private TextView permissionBody;
    private MaterialButton btnPermission;

    // --- map overlays ---
    private Polyline trail;
    private Marker currentMarker;

    // --- session state ---
    private final ArrayList<GeoPoint> trailPoints = new ArrayList<>();
    private final ArrayList<Float> speeds = new ArrayList<>();
    private final Deque<Event> events = new ArrayDeque<>();

    private boolean tracking;
    /** A start was requested but the service has not confirmed it is running yet. */
    private boolean startRequested;
    private boolean darkMap;
    private boolean deviceReady;
    private boolean fineGranted;
    private boolean permanentlyDenied;

    private int fixes;
    private double distanceM;
    private double movingSec;
    private double idleSec;
    private double silentSec;
    private long lastFixWallMs;
    private long lastFixRowAt;
    private long sessionStartMs;
    private double lastLat = Double.NaN;
    private double lastLon = Double.NaN;
    private float lastSpeedKmh;
    private float lastAccuracy = -1f;

    private final Handler ticker = new Handler(Looper.getMainLooper());
    private final Runnable tick = new Runnable() {
        @Override
        public void run() {
            renderLiveTimes();
            ticker.postDelayed(this, 1000);
        }
    };

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
            float speed = intent.getFloatExtra(LocationTrackingService.EXTRA_SPEED, 0f);
            float accuracy = intent.getFloatExtra(LocationTrackingService.EXTRA_ACCURACY, -1f);
            onFix(lat, lon, speed, accuracy);
        }
    };

    // --- permission launchers (must be registered before the activity resumes) ---

    private final ActivityResultLauncher<String> notifLauncher =
            registerForActivityResult(new ActivityResultContracts.RequestPermission(),
                    granted -> requestBackgroundExtras());

    private final ActivityResultLauncher<String> backgroundLauncher =
            registerForActivityResult(new ActivityResultContracts.RequestPermission(),
                    granted -> maybePromptBatteryOptimization());

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

        fineGranted = hasFineLocation();
        renderPermissionState();
        renderAll();

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
        btnMyLocation.setOnClickListener(v -> recentre());

        if (!deviceReady) {
            startEnrollment();
        }

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
        deviceName = findViewById(R.id.deviceName);
        chipLive = findViewById(R.id.chipLive);
        statusText = findViewById(R.id.statusText);
        lastFixText = findViewById(R.id.lastFixText);
        coordsText = findViewById(R.id.coordsText);
        accuracyChip = findViewById(R.id.accuracyChip);
        accuracyText = findViewById(R.id.accuracyText);
        speedValue = findViewById(R.id.speedValue);
        distanceValue = findViewById(R.id.distanceValue);
        fixesValue = findViewById(R.id.fixesValue);
        chartPeak = findViewById(R.id.chartPeak);
        chartEmpty = findViewById(R.id.chartEmpty);
        mapChip = findViewById(R.id.mapChip);
        mapEmpty = findViewById(R.id.mapEmpty);
        donutTotal = findViewById(R.id.donutTotal);
        donutUnit = findViewById(R.id.donutUnit);
        legendMovingPct = findViewById(R.id.legendMovingPct);
        legendIdlePct = findViewById(R.id.legendIdlePct);
        legendSilentPct = findViewById(R.id.legendSilentPct);
        legendMovingDur = findViewById(R.id.legendMovingDur);
        legendIdleDur = findViewById(R.id.legendIdleDur);
        legendSilentDur = findViewById(R.id.legendSilentDur);
        activityCount = findViewById(R.id.activityCount);
        activityEmpty = findViewById(R.id.activityEmpty);
        activityList = findViewById(R.id.activityList);
        speedChart = findViewById(R.id.speedChart);
        donut = findViewById(R.id.donut);
        btnTracking = findViewById(R.id.btnTracking);
        btnTheme = findViewById(R.id.btnTheme);
        btnMyLocation = findViewById(R.id.btnMyLocation);
        statusDot = findViewById(R.id.statusDot);

        enrollOverlay = findViewById(R.id.enrollOverlay);
        enrollTitle = findViewById(R.id.enrollTitle);
        enrollBody = findViewById(R.id.enrollBody);
        btnRetry = findViewById(R.id.btnRetry);
        permissionOverlay = findViewById(R.id.permissionOverlay);
        permissionBody = findViewById(R.id.permissionBody);
        btnPermission = findViewById(R.id.btnPermission);

        deviceName.setText(String.format(Locale.US, "%s %s",
                Build.MANUFACTURER, Build.MODEL).trim());
    }

    // ------------------------------------------------------------------
    // Fix pipeline + session metrics
    // ------------------------------------------------------------------

    private void onFix(double lat, double lon, float speedKmh, float accuracy) {
        long now = System.currentTimeMillis();
        boolean first = Double.isNaN(lastLat);

        if (first) {
            lastFixWallMs = now;
            addEvent(R.string.event_first_fix, getString(R.string.detail_gps), R.color.success, now);
        } else {
            double dt = (now - lastFixWallMs) / 1000.0;
            if (dt > SIGNAL_GAP_SECONDS) {
                silentSec += dt;
                addEvent(R.string.event_signal_lost,
                        getString(R.string.detail_after_gap, fmtDur(dt)), R.color.error, lastFixWallMs);
                addEvent(R.string.event_signal_back, getString(R.string.detail_gps), R.color.warning, now);
            } else if (lastSpeedKmh > 2f) {
                movingSec += dt;
            } else {
                idleSec += dt;
            }
            if (dt > 0 && dt <= 300) {
                distanceM += haversine(lastLat, lastLon, lat, lon);
            }
            lastFixWallMs = now;
        }

        if (first || now - lastFixRowAt >= 15_000) {
            lastFixRowAt = now;
            addEvent(R.string.event_fix, getString(R.string.detail_speed_accuracy,
                    Math.round(speedKmh), Math.max(0, Math.round(accuracy))), R.color.success, now);
        }

        lastLat = lat;
        lastLon = lon;
        lastSpeedKmh = speedKmh;
        lastAccuracy = accuracy;
        fixes++;

        speeds.add(speedKmh);
        while (speeds.size() > MAX_CHART_SAMPLES) {
            speeds.remove(0);
        }

        trailPoints.add(new GeoPoint(lat, lon));
        while (trailPoints.size() > MAX_TRAIL_POINTS) {
            trailPoints.remove(0);
        }
        if (trail != null) {
            trail.setPoints(new ArrayList<>(trailPoints));
        }
        updateMarker(lat, lon);

        if (first) {
            mapEmpty.setVisibility(View.GONE);
            map.getController().setZoom(17.0);
            map.getController().animateTo(new GeoPoint(lat, lon));
        }

        renderStats();
        renderChart();
        renderDonut();
        renderStatus();
        renderMapChip();
    }

    private void updateMarker(double lat, double lon) {
        if (map == null) {
            return;
        }
        GeoPoint point = new GeoPoint(lat, lon);
        if (currentMarker == null) {
            currentMarker = new Marker(map);
            currentMarker.setAnchor(Marker.ANCHOR_CENTER, Marker.ANCHOR_CENTER);
            Drawable icon = ContextCompat.getDrawable(this, R.drawable.ic_location_on);
            if (icon != null) {
                icon = icon.mutate();
                icon.setTint(ContextCompat.getColor(this, R.color.primary));
                currentMarker.setIcon(icon);
            }
            map.getOverlays().add(currentMarker);
        }
        currentMarker.setPosition(point);
        map.invalidate();
    }

    private void recentre() {
        if (trailPoints.isEmpty()) {
            Toast.makeText(this, R.string.no_fix_yet, Toast.LENGTH_SHORT).show();
            return;
        }
        if (trailPoints.size() == 1) {
            map.getController().setZoom(17.0);
            map.getController().animateTo(trailPoints.get(0));
            return;
        }
        BoundingBox box = BoundingBox.fromGeoPoints(trailPoints);
        map.zoomToBoundingBox(box, true, (int) dp(36));
    }

    private void resetSession() {
        fixes = 0;
        distanceM = 0;
        movingSec = 0;
        idleSec = 0;
        silentSec = 0;
        lastFixWallMs = 0;
        lastFixRowAt = 0;
        lastLat = Double.NaN;
        lastLon = Double.NaN;
        lastSpeedKmh = 0;
        lastAccuracy = -1;
        speeds.clear();
        trailPoints.clear();
        events.clear();
        sessionStartMs = System.currentTimeMillis();

        if (trail != null) {
            trail.setPoints(new ArrayList<>());
        }
        if (currentMarker != null) {
            map.getOverlays().remove(currentMarker);
            currentMarker = null;
        }
        mapEmpty.setVisibility(View.VISIBLE);

        renderStats();
        renderChart();
        renderDonut();
        renderActivity();
    }

    // ------------------------------------------------------------------
    // Rendering
    // ------------------------------------------------------------------

    private void renderAll() {
        renderStatus();
        renderStats();
        renderChart();
        renderDonut();
        renderMapChip();
        renderActivity();
    }

    private void renderStatus() {
        statusText.setText(tracking ? R.string.status_on : R.string.status_off);

        int dotColor;
        if (!tracking) {
            dotColor = R.color.muted;
        } else if (Double.isNaN(lastLat)) {
            dotColor = R.color.warning;
        } else {
            dotColor = R.color.success;
        }
        statusDot.setBackgroundTintList(ColorStateList.valueOf(color(dotColor)));

        boolean live = tracking && lastFixWallMs > 0
                && System.currentTimeMillis() - lastFixWallMs < 60_000;
        chipLive.setText(live ? R.string.chip_live : R.string.chip_off);
        chipLive.setTextColor(color(live ? R.color.primary : R.color.muted));
        chipLive.setBackgroundResource(live ? R.drawable.bg_chip_live : R.drawable.bg_chip);

        if (Double.isNaN(lastLat)) {
            lastFixText.setText(R.string.no_fix_yet);
            coordsText.setText(R.string.coords_placeholder);
        } else {
            coordsText.setText(String.format(Locale.US, "%.5f, %.5f", lastLat, lastLon));
        }
        renderLiveTimes();
    }

    /** Cheap 1 Hz refresh of the relative timestamps. */
    private void renderLiveTimes() {
        if (lastFixWallMs <= 0) {
            if (lastFixText != null) {
                lastFixText.setText(R.string.no_fix_yet);
            }
            return;
        }
        long sec = Math.max(0, (System.currentTimeMillis() - lastFixWallMs) / 1000);
        if (sec < 60) {
            lastFixText.setText(getString(R.string.last_fix_seconds, sec));
        } else if (sec < 3600) {
            lastFixText.setText(getString(R.string.last_fix_minutes, sec / 60));
        } else {
            lastFixText.setText(getString(R.string.last_fix_hours, sec / 3600));
        }
    }

    private void renderStats() {
        speedValue.setText(String.valueOf(Math.round(lastSpeedKmh)));
        distanceValue.setText(String.format(Locale.US, "%.1f", distanceM / 1000));
        fixesValue.setText(String.valueOf(fixes));

        if (lastAccuracy > 0) {
            accuracyChip.setText(getString(R.string.accuracy_value, Math.round(lastAccuracy)));
            accuracyChip.setTextColor(color(R.color.on_surface));
        } else {
            accuracyChip.setText(R.string.accuracy_unknown);
            accuracyChip.setTextColor(color(R.color.muted));
        }

        String accuracy = lastAccuracy > 0
                ? getString(R.string.accuracy_value, Math.round(lastAccuracy))
                : getString(R.string.accuracy_unknown);
        String session = sessionStartMs > 0 && tracking
                ? getString(R.string.session_label,
                fmtDur((System.currentTimeMillis() - sessionStartMs) / 1000.0))
                : getString(R.string.status_off);
        accuracyText.setText(getString(R.string.accuracy_footer, accuracy, session));
    }

    private void renderChart() {
        chartEmpty.setVisibility(speeds.size() < 2 ? View.VISIBLE : View.GONE);
        speedChart.setPoints(speeds);
        float peak = 0;
        for (Float value : speeds) {
            if (value != null && value > peak) {
                peak = value;
            }
        }
        chartPeak.setText(getString(R.string.telemetry_peak, Math.round(peak)));
    }

    private void renderDonut() {
        donut.setData((float) movingSec, (float) idleSec, (float) silentSec);
        double total = movingSec + idleSec + silentSec;

        if (total >= 3600) {
            donutTotal.setText(String.format(Locale.US, "%.1f", total / 3600));
            donutUnit.setText(R.string.donut_hours);
        } else {
            donutTotal.setText(String.valueOf(Math.round(total / 60)));
            donutUnit.setText(R.string.donut_minutes);
        }

        pct(legendMovingPct, movingSec, total);
        pct(legendIdlePct, idleSec, total);
        pct(legendSilentPct, silentSec, total);
        legendMovingDur.setText(fmtDur(movingSec));
        legendIdleDur.setText(fmtDur(idleSec));
        legendSilentDur.setText(fmtDur(silentSec));
    }

    private void pct(TextView view, double part, double total) {
        view.setText(total > 0 ? Math.round(part / total * 100) + "%" : "—");
    }

    private void renderMapChip() {
        boolean hasFix = !trailPoints.isEmpty();
        mapChip.setText(hasFix ? R.string.chip_live_trail : R.string.chip_no_fix);
        mapChip.setTextColor(color(hasFix ? R.color.primary : R.color.muted));
        mapChip.setBackgroundResource(hasFix ? R.drawable.bg_chip_live : R.drawable.bg_chip);
    }

    private void renderActivity() {
        activityList.removeAllViews();
        activityEmpty.setVisibility(events.isEmpty() ? View.VISIBLE : View.GONE);
        activityCount.setText(String.valueOf(events.size()));

        LayoutInflater inflater = LayoutInflater.from(this);
        int index = 0;
        for (Event event : events) {
            View row = inflater.inflate(R.layout.item_activity, activityList, false);
            TextView title = row.findViewById(R.id.eventTitle);
            TextView detail = row.findViewById(R.id.eventDetail);
            TextView time = row.findViewById(R.id.eventTime);
            View dot = row.findViewById(R.id.eventDot);
            View divider = row.findViewById(R.id.rowDivider);

            title.setText(event.title);
            detail.setText(event.detail);
            time.setText(agoShort(event.at));
            dot.setBackgroundTintList(ColorStateList.valueOf(color(event.colorRes)));
            divider.setVisibility(index == 0 ? View.GONE : View.VISIBLE);

            activityList.addView(row);
            index++;
        }
    }

    private void addEvent(int titleRes, String detail, int colorRes, long at) {
        events.addFirst(new Event(getString(titleRes), detail, colorRes, at));
        while (events.size() > MAX_EVENTS) {
            events.removeLast();
        }
        renderActivity();
    }

    private void applyMapTheme() {
        if (map == null || btnTheme == null) {
            return;
        }
        TilesOverlay tiles = map.getOverlayManager().getTilesOverlay();
        if (tiles != null) {
            tiles.setColorFilter(darkMap ? TilesOverlay.INVERT_COLORS : null);
        }
        btnTheme.setImageResource(darkMap
                ? R.drawable.ic_light_mode
                : R.drawable.ic_dark_mode);
        map.invalidate();
    }

    // ------------------------------------------------------------------
    // Map
    // ------------------------------------------------------------------

    private void setUpMap() {
        map.setTileSource(TileSourceFactory.MAPNIK);
        map.setMultiTouchControls(true);
        map.getZoomController().setVisibility(CustomZoomButtonsController.Visibility.NEVER);
        map.getController().setZoom(5.0);
        map.getController().setCenter(new GeoPoint(31.79, -7.09));

        trail = new Polyline();
        trail.getOutlinePaint().setStrokeWidth(dp(4));
        trail.getOutlinePaint().setStrokeCap(android.graphics.Paint.Cap.ROUND);
        trail.getOutlinePaint().setColor(ContextCompat.getColor(this, R.color.primary));
        map.getOverlays().add(trail);

        applyMapTheme();

        if (!trailPoints.isEmpty()) {
            trail.setPoints(new ArrayList<>(trailPoints));
            GeoPoint last = trailPoints.get(trailPoints.size() - 1);
            map.getController().setCenter(last);
            map.getController().setZoom(16.0);
            updateMarker(last.getLatitude(), last.getLongitude());
            mapEmpty.setVisibility(View.GONE);
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
            afterForegroundGranted();
        } else {
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

        resetSession();
        addEvent(R.string.event_tracking_started, getString(R.string.detail_session), R.color.success,
                System.currentTimeMillis());

        Intent i = new Intent(this, LocationTrackingService.class);
        i.setAction(LocationTrackingService.ACTION_START);
        ContextCompat.startForegroundService(this, i);

        startRequested = true;
        tracking = true;
        renderTrackingState();
        renderAll();
        requestBackgroundExtras();
    }

    private void stopTracking() {
        Intent i = new Intent(this, LocationTrackingService.class);
        i.setAction(LocationTrackingService.ACTION_STOP);
        startService(i);

        addEvent(R.string.event_tracking_stopped, getString(R.string.detail_session), R.color.muted,
                System.currentTimeMillis());
        startRequested = false;
        tracking = false;
        renderTrackingState();
        renderAll();
    }

    private void renderTrackingState() {
        if (btnTracking == null) {
            return;
        }
        btnTracking.setText(tracking ? R.string.stop_tracking : R.string.start_tracking);
        btnTracking.setIconResource(tracking
                ? R.drawable.ic_stop
                : R.drawable.ic_location_on);
        btnTracking.setBackgroundTintList(ColorStateList.valueOf(color(
                tracking ? R.color.error : R.color.primary)));
        btnTracking.setTextColor(color(tracking ? R.color.on_error : R.color.on_primary));
        btnTracking.setIconTint(ColorStateList.valueOf(color(
                tracking ? R.color.on_error : R.color.on_primary)));
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

        boolean wasGranted = fineGranted;
        fineGranted = hasFineLocation();
        if (fineGranted) {
            permanentlyDenied = false;
        }
        renderPermissionState();

        tracking = LocationTrackingService.isRunning();
        if (tracking) {
            startRequested = false;
        } else if (startRequested) {
            tracking = true;
        }
        renderTrackingState();
        renderAll();

        registerLocationReceiver();
        ticker.removeCallbacks(tick);
        ticker.post(tick);
    }

    @Override
    protected void onPause() {
        if (map != null) {
            map.onPause();
        }
        ticker.removeCallbacks(tick);
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
        ContextCompat.registerReceiver(this, locationReceiver, filter,
                ContextCompat.RECEIVER_NOT_EXPORTED);
        receiverRegistered = true;
    }

    @Override
    protected void onSaveInstanceState(@NonNull Bundle outState) {
        super.onSaveInstanceState(outState);
        outState.putBoolean(STATE_DARK_MAP, darkMap);
        outState.putBoolean(STATE_TRACKING, tracking);
        outState.putDouble(STATE_DISTANCE, distanceM);
        outState.putInt(STATE_FIXES, fixes);
        outState.putDouble(STATE_MOVING, movingSec);
        outState.putDouble(STATE_IDLE, idleSec);
        outState.putDouble(STATE_SILENT, silentSec);
        outState.putDouble(STATE_LAST_LAT, lastLat);
        outState.putDouble(STATE_LAST_LON, lastLon);
        outState.putFloat(STATE_LAST_SPEED, lastSpeedKmh);
        outState.putFloat(STATE_LAST_ACCURACY, lastAccuracy);
        outState.putLong(STATE_LAST_FIX_AT, lastFixWallMs);
        outState.putLong(STATE_SESSION_START, sessionStartMs);

        double[] lat = new double[trailPoints.size()];
        double[] lon = new double[trailPoints.size()];
        for (int i = 0; i < trailPoints.size(); i++) {
            lat[i] = trailPoints.get(i).getLatitude();
            lon[i] = trailPoints.get(i).getLongitude();
        }
        outState.putDoubleArray(STATE_TRAIL_LAT, lat);
        outState.putDoubleArray(STATE_TRAIL_LON, lon);

        float[] samples = new float[speeds.size()];
        for (int i = 0; i < speeds.size(); i++) {
            samples[i] = speeds.get(i);
        }
        outState.putFloatArray(STATE_SPEEDS, samples);
    }

    private void restoreState(Bundle savedInstanceState) {
        if (savedInstanceState != null) {
            darkMap = savedInstanceState.getBoolean(STATE_DARK_MAP, true);
            tracking = savedInstanceState.getBoolean(STATE_TRACKING, false);
            distanceM = savedInstanceState.getDouble(STATE_DISTANCE, 0);
            fixes = savedInstanceState.getInt(STATE_FIXES, 0);
            movingSec = savedInstanceState.getDouble(STATE_MOVING, 0);
            idleSec = savedInstanceState.getDouble(STATE_IDLE, 0);
            silentSec = savedInstanceState.getDouble(STATE_SILENT, 0);
            lastLat = savedInstanceState.getDouble(STATE_LAST_LAT, Double.NaN);
            lastLon = savedInstanceState.getDouble(STATE_LAST_LON, Double.NaN);
            lastSpeedKmh = savedInstanceState.getFloat(STATE_LAST_SPEED, 0);
            lastAccuracy = savedInstanceState.getFloat(STATE_LAST_ACCURACY, -1);
            lastFixWallMs = savedInstanceState.getLong(STATE_LAST_FIX_AT, 0);
            sessionStartMs = savedInstanceState.getLong(STATE_SESSION_START, 0);

            double[] lat = savedInstanceState.getDoubleArray(STATE_TRAIL_LAT);
            double[] lon = savedInstanceState.getDoubleArray(STATE_TRAIL_LON);
            if (lat != null && lon != null && lat.length == lon.length) {
                for (int i = 0; i < lat.length; i++) {
                    trailPoints.add(new GeoPoint(lat[i], lon[i]));
                }
            }
            float[] samples = savedInstanceState.getFloatArray(STATE_SPEEDS);
            if (samples != null) {
                for (float sample : samples) {
                    speeds.add(sample);
                }
            }
        } else {
            darkMap = true;
        }
        tracking = tracking || LocationTrackingService.isRunning();
    }

    @Override
    protected void onDestroy() {
        io.shutdown();
        if (map != null) {
            map.onDetach();
        }
        super.onDestroy();
    }

    // ------------------------------------------------------------------
    // Helpers
    // ------------------------------------------------------------------

    private int color(int res) {
        return ContextCompat.getColor(this, res);
    }

    private float dp(float value) {
        return value * getResources().getDisplayMetrics().density;
    }

    private static double haversine(double lat1, double lon1, double lat2, double lon2) {
        double r = 6_371_000;
        double dLat = Math.toRadians(lat2 - lat1);
        double dLon = Math.toRadians(lon2 - lon1);
        double a = Math.sin(dLat / 2) * Math.sin(dLat / 2)
                + Math.cos(Math.toRadians(lat1)) * Math.cos(Math.toRadians(lat2))
                * Math.sin(dLon / 2) * Math.sin(dLon / 2);
        return 2 * r * Math.asin(Math.min(1, Math.sqrt(a)));
    }

    private String agoShort(long at) {
        long sec = Math.max(0, (System.currentTimeMillis() - at) / 1000);
        if (sec < 10) {
            return "now";
        }
        if (sec < 60) {
            return sec + " s";
        }
        if (sec < 3600) {
            return (sec / 60) + " min";
        }
        return (sec / 3600) + " h";
    }

    private String fmtDur(double seconds) {
        if (seconds < 60) {
            return Math.round(seconds) + " s";
        }
        if (seconds < 3600) {
            return Math.round(seconds / 60) + " min";
        }
        return String.format(Locale.US, "%.1f h", seconds / 3600);
    }

    private static final class Event {
        final String title;
        final String detail;
        final int colorRes;
        final long at;

        Event(String title, String detail, int colorRes, long at) {
            this.title = title;
            this.detail = detail;
            this.colorRes = colorRes;
            this.at = at;
        }
    }
}
