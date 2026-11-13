package com.trackproj.app.ui;

import android.content.Context;
import android.graphics.Canvas;
import android.graphics.LinearGradient;
import android.graphics.Paint;
import android.graphics.Path;
import android.graphics.Shader;
import android.util.AttributeSet;
import android.view.View;

import androidx.annotation.Nullable;
import androidx.core.content.ContextCompat;

import com.trackproj.app.R;

import java.util.ArrayList;
import java.util.List;

/**
 * Minimal speed sparkline for the dashboard. Mirrors the line chart on the web
 * dashboard: mint stroke, soft gradient fill, three horizontal guides.
 */
public class SparklineView extends View {

    private final Paint line = new Paint(Paint.ANTI_ALIAS_FLAG);
    private final Paint fill = new Paint(Paint.ANTI_ALIAS_FLAG);
    private final Paint guide = new Paint(Paint.ANTI_ALIAS_FLAG);
    private final Paint dot = new Paint(Paint.ANTI_ALIAS_FLAG);
    private final Path linePath = new Path();
    private final Path areaPath = new Path();
    private final List<Float> values = new ArrayList<>();

    public SparklineView(Context context) {
        this(context, null);
    }

    public SparklineView(Context context, @Nullable AttributeSet attrs) {
        this(context, attrs, 0);
    }

    public SparklineView(Context context, @Nullable AttributeSet attrs, int defStyleAttr) {
        super(context, attrs, defStyleAttr);

        line.setStyle(Paint.Style.STROKE);
        line.setStrokeWidth(dp(2));
        line.setStrokeJoin(Paint.Join.ROUND);
        line.setStrokeCap(Paint.Cap.ROUND);
        line.setColor(ContextCompat.getColor(context, R.color.primary));

        fill.setStyle(Paint.Style.FILL);

        guide.setStyle(Paint.Style.STROKE);
        guide.setStrokeWidth(dp(1));
        guide.setColor(ContextCompat.getColor(context, R.color.grid));

        dot.setStyle(Paint.Style.FILL);
        dot.setColor(ContextCompat.getColor(context, R.color.primary));
    }

    /** Feed the whole series (oldest first). Kept small by the caller. */
    public void setPoints(List<Float> points) {
        values.clear();
        values.addAll(points);
        invalidate();
    }

    @Override
    protected void onSizeChanged(int w, int h, int oldw, int oldh) {
        super.onSizeChanged(w, h, oldw, oldh);
        int mint = ContextCompat.getColor(getContext(), R.color.primary);
        fill.setShader(new LinearGradient(0, 0, 0, h,
                (mint & 0x00FFFFFF) | 0x33000000, mint & 0x00FFFFFF, Shader.TileMode.CLAMP));
    }

    @Override
    protected void onDraw(Canvas canvas) {
        super.onDraw(canvas);

        float w = getWidth();
        float h = getHeight();
        float padX = dp(2);
        float padY = dp(6);

        for (int i = 0; i <= 2; i++) {
            float y = padY + (h - padY * 2) * i / 2f;
            canvas.drawLine(padX, y, w - padX, y, guide);
        }

        if (values.size() < 2) {
            return;
        }

        float max = 10f;
        for (Float v : values) {
            if (v != null && v > max) {
                max = v;
            }
        }
        max = (float) (Math.ceil(max / 10f) * 10f);

        float innerW = w - padX * 2;
        float innerH = h - padY * 2;
        linePath.reset();
        areaPath.reset();

        float lastX = padX;
        float lastY = padY + innerH;

        for (int i = 0; i < values.size(); i++) {
            Float sample = values.get(i);
            float value = sample == null ? 0f : Math.max(0f, sample);
            float x = padX + innerW * i / (values.size() - 1f);
            float y = padY + innerH - (value / max) * innerH;
            if (i == 0) {
                linePath.moveTo(x, y);
                areaPath.moveTo(x, padY + innerH);
                areaPath.lineTo(x, y);
            } else {
                linePath.lineTo(x, y);
                areaPath.lineTo(x, y);
            }
            lastX = x;
            lastY = y;
        }

        areaPath.lineTo(lastX, padY + innerH);
        areaPath.close();

        canvas.drawPath(areaPath, fill);
        canvas.drawPath(linePath, line);
        canvas.drawCircle(lastX, lastY, dp(3), dot);
    }

    private float dp(float value) {
        return value * getResources().getDisplayMetrics().density;
    }
}
