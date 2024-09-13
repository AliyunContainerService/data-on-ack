/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
*You may obtain a copy of the License at

*   http://www.apache.org/licenses/LICENSE-2.0

*Unless required by applicable law or agreed to in writing, software
*distributed under the License is distributed on an "AS IS" BASIS,
*WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*See the License for the specific language governing permissions and
*limitations under the License.
*/
    
package com.aliyun.kubeai.utils;

import com.google.common.base.Strings;
import lombok.extern.slf4j.Slf4j;

import java.text.DateFormat;
import java.text.SimpleDateFormat;
import java.time.Instant;
import java.util.Date;
import java.util.TimeZone;

@Slf4j
public class DateUtil {

    private static SimpleDateFormat format = new SimpleDateFormat("yyyy-MM-dd HH:mm:ss");

    public static String getDeltaTime(Date submitTime, Date endTime) {
        long diff = endTime.getTime() - submitTime.getTime();

        long days = diff / (1000 * 60 * 60 * 24); //获取天
        long hours = (diff - days * (1000 * 60 * 60 * 24)) / (1000 * 60 * 60); //获取时
        long minutes = (diff - days * (1000 * 60 * 60 * 24) - hours * (1000 * 60 * 60)) / (1000 * 60); //获取分钟
        long seconds = (diff / 1000 - days * 24 * 60 * 60 - hours * 60 * 60 - minutes * 60); //获取秒
        return String.format("%d天%d小时%d分%d秒", days, hours, minutes, seconds);
    }

    public static String transUTCTime(String utcTime) {
        if (Strings.isNullOrEmpty(utcTime)) {
            return null;
        }
        DateFormat utcFormat = new SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'");
        utcFormat.setTimeZone(TimeZone.getTimeZone("UTC"));

        try {
            Date date = utcFormat.parse(utcTime);

            DateFormat pstFormat = new SimpleDateFormat("yyyy-MM-dd HH:mm:ss");
            //pstFormat.setTimeZone(TimeZone.getTimeZone("PST"));
            String resTime = pstFormat.format(date);
            return resTime;
        } catch (Exception e) {
            log.error("trans to utf time error", e);
            return utcTime;
        }
    }

    public static Date getDateFromUTC(String utcTime) {
        DateFormat utcFormat = new SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'");
        utcFormat.setTimeZone(TimeZone.getTimeZone("UTC"));

        try {
            return utcFormat.parse(utcTime);
        } catch (Exception e) {
            return null;
        }
    }

    public static String formatSecond(long duration) {
        long days = duration / (60 * 60 * 24); //获取天
        long hours = (duration - days * (60 * 60 * 24)) / 3600; //获取时
        long minutes = (duration - days * (60 * 60 * 24) - hours * (60 * 60)) / 60; //获取分钟
        long seconds = (duration - days * 24 * 60 * 60 - hours * 60 * 60 - minutes * 60); //获取秒

        if (days == 0 && hours == 0 && minutes == 0) {
            return String.format("%d秒", seconds);
        } else if (days == 0 && hours == 0) {
            return String.format("%d分%d秒", minutes, seconds);
        } else if (days == 0) {
            return String.format("%d小时%d分%d秒", hours, minutes, seconds);
        } else {
            return String.format("%d天%d小时%d分%d秒", days, hours, minutes, seconds);
        }
    }

    public static String getCurrentTime() {
        SimpleDateFormat format = new SimpleDateFormat("yyyyMMddHHmmss");
        return format.format(new Date());
    }

    /*
    public static Date getCreateTime(long duration) {
        Date now = new Date();
        return new Date(now.getTime() - duration * 1000);
    }
     */

    public static Date unixTimestampToDate(long unixTimestamp) {
        Instant instant = Instant.ofEpochSecond(unixTimestamp);
        return Date.from(instant);
    }

    public static String getFormatDuration(Date date) {
        Date now = new Date();
        long diff = now.getTime() - date.getTime();
        return formatSecond(diff / 1000);
    }

    public static long getDurationSecond(Date startTime, Date endTime) {
        long delta = endTime.getTime() - startTime.getTime();
        return delta / (1000);
    }

    public static String formatTime(Date date) {
        return format.format(date);
    }
}
