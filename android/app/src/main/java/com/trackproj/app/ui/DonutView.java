package com.trackproj.app.ui;

import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Paint;
import android.graphics.RectF;
import android.util.AttributeSet;
import android.view.View;

import androidx.annotation.Nullable;
import androidx.core.content.ContextCompat;

import com.trackproj.app.R;

/**
 * Ring chart used by the dashboard's "Time distribution" card. Only the arcs
 * are drawn here; the centre value lives in the XML overlay so it stays a real
 * TextView with the app's typography.
 */
public class DonutView extends View {

    private final Paint paint = new Paint(Paint.ANTI_ALIAS_FLAG);
    private final RectF oval = new RectF();

    private float moving;
    private float idle;
    private float silent;

    public DonutView(Context context) {
        this(context, null);
    }

    public DonutView(Context context, @Nullable AttributeSet attrs) {
        this(context, attrs, 0);
    }

    public DonutView(Context context, @Nullable AttributeSet attrs, int defStyleAttr) {
        super(context, attrs, defStyleAttr);
        paint.setStyle(Paint.Style.STROKE);
        paint.setStrokeCap(Paint.Cap.BUTT);
    }

    /** Durations in seconds; any unit works as long as all three match. */
    public void setData(float moving, float idle, float silent) {
        this.moving = Math.max(0f, moving);
        this.idle = Math.max(0f, idle);
        this.silent = Math.max(0f, silent);
        invalidate();
    }

    @Override
    protected void onDraw(Canvas canvas) {
        super.onDraw(canvas);

        float stroke = dp(18);
        float size = Math.min(getWidth(), getHeight());
        float cx = getWidth() / 2f;
        float cy = getHeight() / 2f;
        float radius = size / 2f - stroke / 2f - dp(1);

        paint.setStrokeWidth(stroke);

        // Track
        paint.setColor(ContextCompat.getColor(getContext(), R.color.grid));
        canvas.drawCircle(cx, cy, radius, paint);

        float total = moving + idle + silent;
        if (total <= 0f || radius <= 0f) {
            return;
        }

        oval.set(cx - radius, cy - radius, cx + radius, cy + radius);
        float start = -90f;
        start = arc(canvas, start, moving / total, R.color.success);
        start = arc(canvas, start, idle / total, R.color.warning);
        arc(canvas, start, silent / total, R.color.muted);
    }

    private float arc(Canvas canvas, float startAngle, float fraction, int colorRes) {
        float sweep = fraction * 360f;
        if (sweep <= 0.1f) {
            return startAngle;
        }
        paint.setColor(ContextCompat.getColor(getContext(), colorRes));
        // A hair of gap between slices keeps the ring legible at small sizes.
        float gap = sweep > 6f ? 2f : 0f;
        canvas.drawArc(oval, startAngle + gap / 2f, sweep - gap, false, paint);
        return startAngle + sweep;
    }

    private float dp(float value) {
        return value * getResources().getDisplayMetrics().density;
    }
}
