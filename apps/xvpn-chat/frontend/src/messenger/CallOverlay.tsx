import { Mic, MicOff, Phone, PhoneOff, Video, VideoOff } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useChat } from '@chat/messenger/ChatProvider'
import { audioConstraints, useChatSettings } from '@chat/messenger/ChatSettings'
import { ChatIconButton } from '@chat/messenger/chrome'
import { videoConstraints } from '@chat/messenger/media'
import { startRingtone } from '@chat/messenger/sound'
import { hasRTCPeerConnection, openCallInBrowser } from '@chat/messenger/webrtc'

type OfferPayload = {
  from?: number
  to?: number
  call_id?: string
  kind?: 'audio' | 'video'
  sdp?: RTCSessionDescriptionInit
  candidate?: RTCIceCandidateInit
}

const ICE: RTCConfiguration = {
  iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
}

function attachStream(el: HTMLMediaElement | null, stream: MediaStream | null) {
  if (!el) return
  if (el.srcObject !== stream) el.srcObject = stream
  if (stream) void el.play().catch(() => {})
}

export function CallOverlay() {
  const { api, session, callEvent, outgoingCall, clearOutgoing, contacts } = useChat()
  const { settings } = useChatSettings()
  const [phase, setPhase] = useState<'idle' | 'ringing' | 'incoming' | 'active'>('idle')
  const [video, setVideo] = useState(false)
  const [muted, setMuted] = useState(false)
  const [camOff, setCamOff] = useState(false)
  const [peerId, setPeerId] = useState<number | null>(null)
  const [mediaError, setMediaError] = useState<string | null>(null)
  const [mediaTick, setMediaTick] = useState(0)
  const pc = useRef<RTCPeerConnection | null>(null)
  const localStream = useRef<MediaStream | null>(null)
  const remoteStream = useRef<MediaStream | null>(null)
  const localVid = useRef<HTMLVideoElement>(null)
  const remoteVid = useRef<HTMLVideoElement>(null)
  const remoteAud = useRef<HTMLAudioElement>(null)
  const incomingSdp = useRef<RTCSessionDescriptionInit | null>(null)
  const peerIdRef = useRef<number | null>(null)
  const callIdRef = useRef('')

  const peerName =
    contacts.find((c) => c.kind === 'dm' && c.peerUserId === peerId)?.title ?? (peerId ? `#${peerId}` : '')

  function signal(type: string, payload: Record<string, unknown>) {
    api.sendSignal(type, payload)
  }

  function bindRemote(stream: MediaStream) {
    remoteStream.current = stream
    attachStream(remoteVid.current, stream)
    attachStream(remoteAud.current, stream)
  }

  async function ensurePC(withVideo: boolean) {
    if (!hasRTCPeerConnection()) {
      throw new Error('WebRTC indisponível neste WebView — abra a chamada no navegador')
    }
    if (pc.current) return pc.current
    const conn = new RTCPeerConnection(ICE)
    pc.current = conn
    const stream = await navigator.mediaDevices.getUserMedia({
      audio: audioConstraints(settings.micId),
      video: withVideo ? videoConstraints(settings.cameraId) : false,
    })
    localStream.current = stream
    stream.getTracks().forEach((t) => conn.addTrack(t, stream))
    attachStream(localVid.current, stream)
    setMediaTick((n) => n + 1)
    conn.ontrack = (ev) => {
      const [remote] = ev.streams
      if (remote) bindRemote(remote)
    }
    conn.onicecandidate = (ev) => {
      const to = peerIdRef.current
      const id = callIdRef.current
      if (ev.candidate && to && id) {
        signal('call.ice', { to, call_id: id, candidate: ev.candidate.toJSON() })
      }
    }
    return conn
  }

  function hangup(notify = true) {
    if (notify && peerIdRef.current && callIdRef.current) {
      signal('call.hangup', { to: peerIdRef.current, call_id: callIdRef.current })
    }
    localStream.current?.getTracks().forEach((t) => t.stop())
    localStream.current = null
    remoteStream.current = null
    pc.current?.close()
    pc.current = null
    incomingSdp.current = null
    setPhase('idle')
    setPeerId(null)
    peerIdRef.current = null
    callIdRef.current = ''
    setMediaError(null)
    setMuted(false)
    setCamOff(false)
    clearOutgoing()
  }

  useEffect(() => {
    attachStream(localVid.current, localStream.current)
    attachStream(remoteVid.current, remoteStream.current)
    attachStream(remoteAud.current, remoteStream.current)
  }, [phase, video, mediaTick])

  useEffect(() => {
    if (!outgoingCall || !session) return
    const { to, video: v, callId: id } = outgoingCall
    peerIdRef.current = to
    callIdRef.current = id
    setPeerId(to)
    setVideo(v)
    setPhase('ringing')
    setMediaError(null)
    void (async () => {
      try {
        const conn = await ensurePC(v)
        const offer = await conn.createOffer()
        await conn.setLocalDescription(offer)
        signal('call.offer', { to, call_id: id, kind: v ? 'video' : 'audio', sdp: offer })
      } catch (e) {
        setMediaError(e instanceof Error ? e.message : 'Não foi possível abrir microfone/câmera')
      }
    })()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [outgoingCall?.callId])

  useEffect(() => {
    if (!callEvent?.payload || typeof callEvent.payload !== 'object') return
    const p = callEvent.payload as OfferPayload
    if (callEvent.type === 'call.offer' && p.from && p.call_id && p.sdp) {
      peerIdRef.current = p.from
      callIdRef.current = p.call_id
      setPeerId(p.from)
      setVideo(p.kind === 'video')
      setPhase('incoming')
      incomingSdp.current = p.sdp
    }
    if (callEvent.type === 'call.answer' && p.sdp && pc.current) {
      void pc.current.setRemoteDescription(p.sdp)
      setPhase('active')
    }
    if (callEvent.type === 'call.ice' && p.candidate && pc.current) {
      void pc.current.addIceCandidate(p.candidate)
    }
    if (callEvent.type === 'call.hangup' || callEvent.type === 'call.reject') {
      hangup(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [callEvent])

  useEffect(() => {
    if ((phase !== 'incoming' && phase !== 'ringing') || !settings.soundCall) return
    return startRingtone(settings.volume)
  }, [phase, settings.soundCall, settings.volume])

  async function accept() {
    if (!peerIdRef.current || !incomingSdp.current) return
    try {
      const conn = await ensurePC(video)
      await conn.setRemoteDescription(incomingSdp.current)
      const answer = await conn.createAnswer()
      await conn.setLocalDescription(answer)
      signal('call.answer', { to: peerIdRef.current, call_id: callIdRef.current, sdp: answer })
      setPhase('active')
    } catch (e) {
      setMediaError(e instanceof Error ? e.message : 'Não foi possível atender')
    }
  }

  return (
    <>
      <audio ref={remoteAud} autoPlay playsInline className="hidden" />
      {phase !== 'idle' && (
        <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/70 p-4">
          <div className="relative w-full max-w-sm rounded-[22px] p-5 watch-complication">
            {video && (
              <div className="relative mb-4 overflow-hidden rounded-[18px] bg-black">
                <video ref={remoteVid} autoPlay playsInline className="h-56 w-full bg-black object-cover" />
                <video
                  ref={localVid}
                  autoPlay
                  muted
                  playsInline
                  className="absolute bottom-2 right-2 z-10 h-20 w-16 rounded-[10px] bg-black object-cover ring-1 ring-white/20"
                />
              </div>
            )}
            <p className="text-center font-display text-[11px] uppercase tracking-[0.14em] text-muted-foreground/75">
              {phase === 'incoming' ? 'Chamada recebida' : phase === 'ringing' ? 'Chamando…' : video ? 'Vídeo' : 'Áudio'}
            </p>
            <p className="mt-1 text-center font-display text-xl font-semibold">{peerName}</p>
            {mediaError && <p className="mt-2 text-center text-xs text-destructive">{mediaError}</p>}
            {!hasRTCPeerConnection() && (
              <div className="mt-3 space-y-2 text-center">
                <p className="text-xs text-muted-foreground">
                  Este app (WebKit) não tem WebRTC. A chamada abre no Chromium, na VPN.
                </p>
                <ChatIconButton label="Abrir chamada no navegador" filled onClick={openCallInBrowser}>
                  <Phone className="h-4 w-4 text-[var(--safe)]" />
                </ChatIconButton>
              </div>
            )}
            <div className="mt-5 flex justify-center gap-3">
              {phase === 'incoming' ? (
                <>
                  <ChatIconButton label="Recusar" filled onClick={() => hangup(true)}>
                    <PhoneOff className="h-4 w-4 text-destructive" />
                  </ChatIconButton>
                  <ChatIconButton label="Atender" filled onClick={() => void accept()}>
                    <Phone className="h-4 w-4 text-[var(--safe)]" />
                  </ChatIconButton>
                </>
              ) : (
                <>
                  <ChatIconButton
                    label={muted ? 'Ativar microfone' : 'Mudo'}
                    filled
                    onClick={() => {
                      const next = !muted
                      setMuted(next)
                      localStream.current?.getAudioTracks().forEach((t) => {
                        t.enabled = !next
                      })
                    }}
                  >
                    {muted ? <MicOff className="h-4 w-4" /> : <Mic className="h-4 w-4" />}
                  </ChatIconButton>
                  {video && (
                    <ChatIconButton
                      label="Câmera"
                      filled
                      onClick={() => {
                        const next = !camOff
                        setCamOff(next)
                        localStream.current?.getVideoTracks().forEach((t) => {
                          t.enabled = !next
                        })
                      }}
                    >
                      {camOff ? <VideoOff className="h-4 w-4" /> : <Video className="h-4 w-4" />}
                    </ChatIconButton>
                  )}
                  <ChatIconButton label="Encerrar" filled onClick={() => hangup(true)}>
                    <PhoneOff className="h-4 w-4 text-destructive" />
                  </ChatIconButton>
                </>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  )
}
