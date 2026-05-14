ARG SPARK_IMAGE=spark:3.5.8

FROM ${SPARK_IMAGE}

# Add dependency for Hadoop Aliyun OSS support
ADD --chown=spark:spark --chmod=644 https://repo1.maven.org/maven2/org/apache/hadoop/hadoop-aliyun/3.3.4/hadoop-aliyun-3.3.4.jar ${SPARK_HOME}/jars
ADD --chown=spark:spark --chmod=644 https://repo1.maven.org/maven2/com/aliyun/oss/aliyun-sdk-oss/3.17.4/aliyun-sdk-oss-3.17.4.jar ${SPARK_HOME}/jars
ADD --chown=spark:spark --chmod=644 https://repo1.maven.org/maven2/org/jdom/jdom2/2.0.6.1/jdom2-2.0.6.1.jar ${SPARK_HOME}/jars