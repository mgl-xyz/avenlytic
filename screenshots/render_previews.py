from PIL import Image, ImageDraw, ImageFont
from pathlib import Path
OUT=Path(__file__).parent
W,H=1440,960
BG='#0a111d'; PANEL='#111e2e'; PANEL2='#142438'; BORDER='#203249'; TEXT='#e8eef7'; MUTED='#8296ad'; DIM='#617991'; TEAL='#5bd3bd'; GREEN='#62d8b0'; RED='#ff9298'; ORANGE='#f0b477'; BLUE='#75c7ee'; PURPLE='#b9a5fb'
font_cjk='/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc'; font_lat='/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf'
def F(sz,bold=False): return ImageFont.truetype(font_cjk,sz,index=2)
def base(title='基础设施总览',active='总览',user='admin'):
 im=Image.new('RGB',(W,H),BG);d=ImageDraw.Draw(im)
 d.rectangle((0,0,246,H),fill='#0d1725');d.line((245,0,245,H),fill='#1b2a3c',width=1)
 d.rounded_rectangle((26,26,66,66),12,fill='#258e8b');d.text((37,29),'◉',font=F(26,True),fill='#08221f')
 d.text((78,29),'AVENLYTIC',font=F(14,True),fill=TEXT);d.text((78,50),'INFRASTRUCTURE',font=F(8),fill=DIM)
 d.text((30,106),'WORKSPACE',font=F(9,True),fill=DIM)
 nav=[('◫','总览'),('▣','服务器'),('⚑','告警中心'),('⌁','告警规则'),('↗','通知通道'),('⚙','系统设置')]
 for i,(ic,n) in enumerate(nav):
  y=137+i*49
  if n==active:d.rounded_rectangle((15,y-7,230,y+32),8,fill='#173b40')
  d.text((31,y),ic,font=F(16),fill=TEAL if n==active else MUTED);d.text((62,y+3),n,font=F(12),fill=TEAL if n==active else MUTED)
 d.line((20,880,225,880),fill=BORDER);d.ellipse((29,901,37,909),fill=GREEN);d.text((48,897),'监控引擎运行中',font=F(10),fill='#83a096');d.text((48,918),'Agent protocol · v1',font=F(8),fill=DIM)
 d.text((284,28),'AVENLYTIC   /   '+title,font=F(9,True),fill=DIM)
 d.text((284,53),title,font=F(25,True),fill=TEXT);d.text((284,91),'实时掌握基础设施运行状态与告警情况。',font=F(11),fill=MUTED)
 d.line((284,124,1405,124),fill=BORDER)
 d.rounded_rectangle((1124,48,1227,83),8,fill='#142235',outline=BORDER);d.text((1140,59),'◉  实时更新',font=F(10),fill='#82a194')
 d.rounded_rectangle((1240,48,1368,83),8,fill='#142235',outline=BORDER);d.ellipse((1250,54,1276,80),fill='#234b53');d.text((1258,57),user[0].upper(),font=F(12,True),fill='#83e2d1');d.text((1285,59),user,font=F(10),fill='#a8bbce')
 return im,d
def card(d,xy,fill=PANEL,outline=BORDER,r=10):d.rounded_rectangle(xy,r,fill=fill,outline=outline,width=1)
def txt(d,xy,s,size=11,color=TEXT,bold=False):d.text(xy,s,font=F(size,bold),fill=color)
def login():
 im=Image.new('RGB',(W,H),BG);d=ImageDraw.Draw(im)
 # subtle radial-style bands
 d.ellipse((390,-300,1100,850),fill='#102a3a');d.ellipse((500,-180,1000,700),fill='#102234')
 d.rounded_rectangle((505,220,935,745),16,fill='#111f30',outline='#2a4057',width=1)
 d.rounded_rectangle((544,255,590,301),13,fill='#258e8b');txt(d,(556,260),'◉',29,'#08221f',True);txt(d,(604,258),'AVENLYTIC',14,TEXT,True);txt(d,(604,279),'LIGHTWEIGHT INFRASTRUCTURE MONITOR',8,DIM)
 txt(d,(545,345),'FIRST-TIME SETUP',9,TEAL,True);txt(d,(545,372),'创建管理员账号',25,TEXT,True);txt(d,(545,414),'完成初始化后即可通过浏览器管理所有监控配置。',10,MUTED)
 for label,y,val in [('管理员用户名',466,'admin'),('设置密码',550,'••••••••••••••••'),('确认密码',634,'••••••••••••••••')]:
  txt(d,(545,y),label,10,'#a5b7c9');card(d,(545,y+22,895,y+62),'#0b1725','#30465e',7);txt(d,(560,y+34),val,11,'#dce7f2')
 d.rounded_rectangle((545,710,895,754),7,fill=TEAL);txt(d,(625,723),'创建管理员并进入系统',11,'#08231f',True)
 txt(d,(545,780),'●  HttpOnly Cookie  ·  受保护的管理会话',9,DIM);txt(d,(545,835),'Avenlytic · Observe everything, stay lightweight.',9,DIM)
 return im
def dashboard():
 im,d=base('基础设施总览','总览')
 stats=[('服务器总数','28','已注册 Agent',BLUE,'▣'),('在线服务器','26','92.8% 当前在线',GREEN,'◉'),('平均 CPU','18.6%','在线主机平均值',PURPLE,'⌁'),('平均内存','62.4%','在线主机平均值',ORANGE,'▤')]
 for i,(label,val,foot,col,ic) in enumerate(stats):
  x=284+i*280;card(d,(x,151,x+263,282), '#142337');txt(d,(x+17,169),label,10,MUTED);txt(d,(x+17,199),val,29,TEXT if i!=1 else GREEN,True);txt(d,(x+17,246),foot,9,MUTED);d.rounded_rectangle((x+208,226,x+246,264),9,fill='#1c3b52');txt(d,(x+218,232),ic,18,col,True)
 txt(d,(284,316),'服务器状态',15,TEXT,True);txt(d,(284,342),'最近一次上报的主机指标',10,MUTED)
 card(d,(284,370,1405,670));d.rectangle((285,371,1404,411),fill=PANEL2)
 headers=[('服务器',303),('平台',495),('CPU',655),('内存',790),('磁盘',930),('进程',1068),('最后上报',1170),('状态',1320)]
 for s,x in headers:txt(d,(x,385),s,9,MUTED,True)
 rows=[('web-prod-01','linux · amd64',12.8,48.2,56.1,'184','5 秒前','在线',GREEN),('db-primary-01','linux · amd64',76.3,81.4,72.8,'246','4 秒前','在线',ORANGE),('cache-01','linux · arm64',8.2,34.7,41.2,'62','6 秒前','在线',GREEN),('build-runner-02','linux · amd64',0,0,0,'—','3 分钟前','离线',RED)]
 for j,row in enumerate(rows):
  y=428+j*58
  if j%2==0:d.rectangle((285,y-7,1404,y+42),fill='#132235')
  host,plat,cpu,mem,disk,proc,last,statusv,col=row
  txt(d,(303,y),host,10,TEXT,True);txt(d,(303,y+17),'agent-'+str(j+1)+' · 8d1a…',8,DIM);txt(d,(495,y+5),plat,9,MUTED)
  for xx,v in [(655,cpu),(790,mem),(930,disk)]:
   txt(d,(xx,y),f'{v:.1f}%',9,TEXT);d.rounded_rectangle((xx,y+19,xx+92,y+24),3,fill='#26384d');d.rounded_rectangle((xx,y+19,xx+92*v/100,y+24),3,fill=RED if v>85 else ORANGE if v>75 else TEAL)
  txt(d,(1068,y+5),proc,9,MUTED);txt(d,(1170,y+5),last,9,MUTED);d.rounded_rectangle((1317,y,1371,y+23),12,fill='#153d35' if statusv=='在线' else '#482a32');txt(d,(1328,y+5),statusv,9,col)
 txt(d,(284,709),'最近告警',15,TEXT,True);txt(d,(284,735),'需要关注的事件',10,MUTED)
 card(d,(284,761,890,925));card(d,(908,761,1405,925))
 for i,(t,s) in enumerate([('磁盘使用率超过 85% · db-primary-01','/var/lib/postgresql 使用率 88.2%'),('Agent 心跳超时 · build-runner-02','最后上报时间超过 180 秒')]):
  y=787+i*60;d.ellipse((304,y+4,312,y+12),fill=ORANGE if i==0 else RED);txt(d,(326,y),t,10,TEXT,True);txt(d,(326,y+23),s,9,MUTED)
 txt(d,(932,787),'快速接入',12,TEXT,True);txt(d,(932,815),'LINUX AGENT',8,DIM,True);txt(d,(932,843),'在系统设置中生成 Agent Token，',10,MUTED);txt(d,(932,864),'复制安装命令并部署到目标服务器。',10,MUTED)
 return im
def rules_page():
 im,d=base('告警规则','告警规则')
 txt(d,(284,157),'规则配置',15,TEXT,True);txt(d,(284,182),'定义阈值、持续时间、通知通道和恢复行为。',10,MUTED)
 d.rounded_rectangle((1250,151,1405,188),7,fill=TEAL);txt(d,(1272,163),'＋ 新建规则',10,'#08231f',True)
 card(d,(284,216,1405,750));d.rectangle((285,217,1404,260),fill=PANEL2)
 for label,x in [('规则名称',303),('监控指标',570),('触发条件',815),('持续时间',980),('级别',1095),('状态',1190),('操作',1310)]:txt(d,(x,232),label,9,MUTED,True)
 rows=[('内存使用率过高','memory.usage_percent','> 85%','60 秒','警告','启用'),('CPU 持续高负载','cpu.usage_percent','> 90%','120 秒','严重','启用'),('磁盘空间不足','disk.usage_percent','> 88%','30 秒','严重','启用'),('Agent 离线检测','agent.online','== 0','0 秒','严重','启用'),('系统负载异常','load.1m','> 8','180 秒','警告','停用')]
 for i,r in enumerate(rows):
  y=282+i*77
  if i%2==0:d.rectangle((285,y-8,1404,y+51),fill='#132235')
  for (v,x) in zip(r[:5],[303,570,815,980,1095]):txt(d,(x,y+6),v,10,TEXT if x==303 else MUTED,x==303)
  d.rounded_rectangle((1185,y,1240,y+25),12,fill='#153d35' if r[5]=='启用' else '#482a32');txt(d,(1198,y+6),r[5],9,GREEN if r[5]=='启用' else RED)
  txt(d,(1305,y+6),'编辑   删除',9,TEAL)
 card(d,(284,780,1405,912),'#142438','#2d4053');txt(d,(305,800),'规则说明',11,TEXT,True);txt(d,(305,830),'持续时间用于抑制瞬时尖峰误报。Agent 离线规则由中心端心跳检测器执行。',10,MUTED);txt(d,(305,856),'可为每条规则绑定多个通知通道，并单独控制恢复通知。',10,MUTED)
 return im
def channels_page():
 im,d=base('通知通道','通知通道')
 txt(d,(284,157),'通知通道',15,TEXT,True);txt(d,(284,182),'统一管理告警接收渠道，支持测试发送和投递追踪。',10,MUTED)
 d.rounded_rectangle((1250,151,1405,188),7,fill=TEAL);txt(d,(1273,163),'＋ 添加通道',10,'#08231f',True)
 items=[('钉','运维钉钉群','DINGTALK','启用'),('飞','基础设施告警','FEISHU','启用'),('✉','On-call 邮件','SMTP EMAIL','启用'),('企','值班企业微信群','WECOM','启用'),('↗','自定义告警 Webhook','WEBHOOK','停用'),('T','Telegram Ops','TELEGRAM','启用')]
 for i,(ic,name,typ,state) in enumerate(items):
  x=284+(i%3)*376;y=218+(i//3)*154;card(d,(x,y,x+356,y+134));d.rounded_rectangle((x+17,y+17,x+54,y+54),9,fill='#1e3a4e');txt(d,(x+27,y+23),ic,17,TEAL,True);d.rounded_rectangle((x+277,y+22,x+337,y+44),12,fill='#153d35' if state=='启用' else '#482a32');txt(d,(x+289,y+28),state,8,GREEN if state=='启用' else RED);txt(d,(x+18,y+72),name,12,TEXT,True);txt(d,(x+18,y+96),typ,8,DIM,True);d.line((x+18,y+116,x+338,y+116),fill=BORDER);txt(d,(x+18,y+120),'编辑     测试发送     删除',8,TEAL)
 txt(d,(284,548),'最近投递记录',15,TEXT,True);txt(d,(284,574),'通知发送结果与失败原因',10,MUTED)
 card(d,(284,602,1405,900));d.rectangle((285,603,1404,645),fill=PANEL2)
 for s,x in [('通道',303),('类型',550),('状态',720),('尝试次数',865),('时间',1010),('错误',1210)]:txt(d,(x,618),s,9,MUTED,True)
 for i,r in enumerate([('运维钉钉群','DINGTALK','成功','1','10:42:18','—'),('On-call 邮件','SMTP','成功','1','10:38:02','—'),('值班企业微信群','WECOM','失败','3','10:21:47','connection timeout')]):
  y=667+i*65
  for v,x in zip(r,[303,550,720,865,1010,1210]):txt(d,(x,y),v,9,GREEN if v=='成功' else RED if v=='失败' else MUTED)
 return im
def settings_page():
 im,d=base('系统设置','系统设置')
 txt(d,(284,157),'实例配置',15,TEXT,True);txt(d,(284,182),'配置 Agent 接入、安全策略和管理员账户。',10,MUTED)
 card(d,(284,218,835,570));txt(d,(307,242),'Agent 接入配置',13,TEXT,True);txt(d,(307,270),'Agent 使用此 Token 主动连接中心端。修改后旧 Token 立即失效。',9,MUTED);txt(d,(307,311),'Agent 通信 Token',10,'#9cb0c5');card(d,(307,335,805,378),'#0b1725','#2b4057',6);txt(d,(321,349),'••••••••••••••••••••••••••••••••',11,DIM);d.rounded_rectangle((713,341,790,371),6,fill='#1b3348');txt(d,(731,350),'生成',9,TEAL);txt(d,(307,398),'当前状态：已配置 · 密钥不会回显',9,GREEN);txt(d,(307,424),'默认采集间隔（秒）',10,'#9cb0c5');card(d,(307,445,805,483),'#0b1725','#2b4057',6);txt(d,(321,455),'5',11,TEXT);txt(d,(307,495),'Agent 离线判定时间（秒）',10,'#9cb0c5');card(d,(307,516,805,554),'#0b1725','#2b4057',6);txt(d,(321,526),'45',11,TEXT);d.rounded_rectangle((307,568,431,602),7,fill=TEAL);txt(d,(337,579),'保存系统设置',9,'#08231f',True)
 card(d,(855,218,1405,570));txt(d,(879,242),'快速部署 Linux Agent',13,TEXT,True);txt(d,(879,270),'从 GitHub Releases 下载对应架构的 Agent 二进制。',9,MUTED);card(d,(879,306,1380,390),'#0b1725','#2b4057',7);txt(d,(895,324),'./avenlytic-agent-linux-amd64 --server https://YOUR_SERVER:9080',9,'#81d7c8');txt(d,(895,350),'  --token YOUR_AGENT_TOKEN --interval 5s',9,'#81d7c8');txt(d,(879,415),'Agent 主动上报，不需要开放服务器入站端口。',9,MUTED);d.rounded_rectangle((879,468,1025,505),7,fill='#1b3348');txt(d,(902,481),'复制部署命令',9,TEAL)
 card(d,(284,594,835,864));txt(d,(307,618),'管理员账户',13,TEXT,True);txt(d,(307,654),'当前登录用户',9,MUTED);txt(d,(307,679),'admin',12,TEXT,True);d.rounded_rectangle((307,729,430,766),7,fill='#1b3348');txt(d,(331,741),'修改密码',9,TEAL);d.rounded_rectangle((444,729,555,766),7,fill='#1b3348');txt(d,(468,741),'退出登录',9,TEAL)
 card(d,(855,594,1405,864));txt(d,(879,618),'实例信息',13,TEXT,True)
 for i,(a,b) in enumerate([('产品版本','0.4.0'),('Agent 协议','JSON / v1'),('数据存储','JSON 单节点存储'),('前端','Vue 3 + Vite')]):
  y=665+i*43;txt(d,(879,y),a,9,MUTED);txt(d,(1225,y),b,9,TEXT)
 return im

for name,fn in [('01-first-run-setup.png',login),('02-dashboard.png',dashboard),('03-alert-rules.png',rules_page),('04-notification-channels.png',channels_page),('05-system-settings.png',settings_page)]:
 img=fn();img.save(OUT/name,optimize=True)
 print(name,img.size)
